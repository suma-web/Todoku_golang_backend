package attachment

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
)

var (
	ErrValidation = errors.New("invalid attachment")
	ErrForbidden  = errors.New("attachment forbidden")
	ErrNotFound   = errors.New("attachment not found")
)

var allowedTypes = map[string]string{
	"application/pdf": ".pdf",
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/webp":      ".webp",
}

type Service struct {
	db      *sql.DB
	storage Storage
}

func NewService(db *sql.DB, storage Storage) *Service {
	return &Service{db: db, storage: storage}
}

func (s *Service) Upload(ctx context.Context, userID int64, parent ParentType, parentID int64, headers []*multipart.FileHeader) ([]File, error) {
	if parentID < 1 || len(headers) == 0 {
		return nil, ErrValidation
	}
	allowed, err := s.canUpload(ctx, userID, parent, parentID)
	if err != nil || !allowed {
		return nil, ErrForbidden
	}
	existingCount, existingBytes, err := s.usage(ctx, parent, parentID)
	if err != nil {
		return nil, err
	}
	if existingCount+len(headers) > MaxFilesPerParent {
		return nil, ErrValidation
	}
	var incomingBytes int64
	for _, header := range headers {
		if header.Size < 1 || header.Size > MaxFileBytes {
			return nil, ErrValidation
		}
		incomingBytes += header.Size
	}
	if existingBytes+incomingBytes > MaxTotalBytes {
		return nil, ErrValidation
	}

	items := make([]File, 0, len(headers))
	for _, header := range headers {
		item, err := s.uploadOne(ctx, userID, parent, parentID, header)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) uploadOne(ctx context.Context, userID int64, parent ParentType, parentID int64, header *multipart.FileHeader) (File, error) {
	file, err := header.Open()
	if err != nil {
		return File{}, ErrValidation
	}
	defer file.Close()
	first := make([]byte, 512)
	n, err := io.ReadFull(file, first)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return File{}, ErrValidation
	}
	contentType := http.DetectContentType(first[:n])
	extension, ok := allowedTypes[contentType]
	if !ok {
		return File{}, ErrValidation
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return File{}, err
	}
	key, err := storageKey(extension)
	if err != nil {
		return File{}, err
	}
	if err := s.storage.Put(ctx, key, contentType, file); err != nil {
		return File{}, fmt.Errorf("store attachment: %w", err)
	}
	item := File{OriginalName: safeName(header.Filename), ContentType: contentType, SizeBytes: header.Size, storageKey: key}
	column := map[ParentType]string{SchoolPostParent: "school_post_id", QuestionParent: "question_id", AnswerParent: "answer_id"}[parent]
	if column == "" {
		_ = s.storage.Delete(ctx, key)
		return File{}, ErrValidation
	}
	query := fmt.Sprintf(`INSERT INTO attachments(uploaded_by,%s,original_name,storage_key,content_type,size_bytes) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,created_at`, column)
	if err := s.db.QueryRowContext(ctx, query, userID, parentID, item.OriginalName, key, contentType, item.SizeBytes).Scan(&item.ID, &item.CreatedAt); err != nil {
		_ = s.storage.Delete(ctx, key)
		return File{}, err
	}
	return item, nil
}

func (s *Service) List(ctx context.Context, userID int64, parent ParentType, parentID int64) ([]File, error) {
	allowed, err := s.canReadParent(ctx, userID, parent, parentID)
	if err != nil || !allowed {
		return nil, ErrForbidden
	}
	column := map[ParentType]string{SchoolPostParent: "school_post_id", QuestionParent: "question_id", AnswerParent: "answer_id"}[parent]
	if column == "" {
		return nil, ErrValidation
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT id,original_name,content_type,size_bytes,created_at FROM attachments WHERE %s=$1 ORDER BY created_at,id`, column), parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []File{}
	for rows.Next() {
		var item File
		if err := rows.Scan(&item.ID, &item.OriginalName, &item.ContentType, &item.SizeBytes, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Download(ctx context.Context, userID, attachmentID int64) (File, io.ReadCloser, error) {
	var item File
	var schoolPostID, questionID, answerID sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT id,original_name,storage_key,content_type,size_bytes,created_at,school_post_id,question_id,answer_id FROM attachments WHERE id=$1`, attachmentID).Scan(
		&item.ID, &item.OriginalName, &item.storageKey, &item.ContentType, &item.SizeBytes, &item.CreatedAt, &schoolPostID, &questionID, &answerID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, nil, ErrNotFound
	}
	if err != nil {
		return File{}, nil, err
	}
	parent, parentID := SchoolPostParent, schoolPostID.Int64
	if questionID.Valid {
		parent, parentID = QuestionParent, questionID.Int64
	}
	if answerID.Valid {
		parent, parentID = AnswerParent, answerID.Int64
	}
	allowed, err := s.canReadParent(ctx, userID, parent, parentID)
	if err != nil || !allowed {
		return File{}, nil, ErrForbidden
	}
	body, err := s.storage.Get(ctx, item.storageKey)
	return item, body, err
}

func (s *Service) usage(ctx context.Context, parent ParentType, parentID int64) (int, int64, error) {
	column := map[ParentType]string{SchoolPostParent: "school_post_id", QuestionParent: "question_id", AnswerParent: "answer_id"}[parent]
	if column == "" {
		return 0, 0, ErrValidation
	}
	var count int
	var total int64
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*),COALESCE(SUM(size_bytes),0) FROM attachments WHERE %s=$1`, column), parentID).Scan(&count, &total)
	return count, total, err
}

func (s *Service) canUpload(ctx context.Context, userID int64, parent ParentType, parentID int64) (bool, error) {
	query := ""
	switch parent {
	case SchoolPostParent:
		query = `SELECT EXISTS(SELECT 1 FROM school_posts p JOIN users u ON u.id=$1 WHERE p.id=$2 AND(p.author_id=$1 OR u.role='admin'))`
	case QuestionParent:
		query = `SELECT EXISTS(SELECT 1 FROM questions q JOIN users u ON u.id=$1 WHERE q.id=$2 AND(q.user_id=$1 OR u.role='admin'))`
	case AnswerParent:
		query = `SELECT EXISTS(SELECT 1 FROM question_answers a JOIN users u ON u.id=$1 WHERE a.id=$2 AND(a.user_id=$1 OR u.role='admin'))`
	default:
		return false, ErrValidation
	}
	var allowed bool
	err := s.db.QueryRowContext(ctx, query, userID, parentID).Scan(&allowed)
	return allowed, err
}

func (s *Service) canReadParent(ctx context.Context, userID int64, parent ParentType, parentID int64) (bool, error) {
	if parent == AnswerParent {
		var questionID int64
		if err := s.db.QueryRowContext(ctx, `SELECT question_id FROM question_answers WHERE id=$1`, parentID).Scan(&questionID); err != nil {
			return false, err
		}
		parent, parentID = QuestionParent, questionID
	}
	query := ""
	switch parent {
	case SchoolPostParent:
		query = `SELECT EXISTS(SELECT 1 FROM school_posts p JOIN users u ON u.id=$1 WHERE p.id=$2 AND(p.author_id=$1 OR u.role IN('teacher','admin') OR EXISTS(SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id WHERE pg.post_id=p.id AND ug.user_id=$1)))`
	case QuestionParent:
		query = `SELECT EXISTS(SELECT 1 FROM questions q JOIN question_categories c ON c.id=q.category_id JOIN users u ON u.id=$1 WHERE q.id=$2 AND(q.user_id=$1 OR q.visibility='public' OR u.role='admin' OR(u.role='teacher' AND EXISTS(SELECT 1 FROM user_school_groups ug WHERE ug.user_id=$1 AND ug.group_id=c.group_id))))`
	default:
		return false, ErrValidation
	}
	var allowed bool
	err := s.db.QueryRowContext(ctx, query, userID, parentID).Scan(&allowed)
	return allowed, err
}

func storageKey(extension string) (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "attachments/" + hex.EncodeToString(value) + extension, nil
}

func safeName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." {
		return "attachment"
	}
	runes := []rune(name)
	if len(runes) > 255 {
		return string(runes[:255])
	}
	return name
}
