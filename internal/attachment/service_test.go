package attachment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type storageStub struct {
	putKeys    []string
	deleteKeys []string
	getCalls   int
}

func (s *storageStub) Put(_ context.Context, key, _ string, _ io.Reader) error {
	s.putKeys = append(s.putKeys, key)
	return nil
}

func (s *storageStub) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	s.getCalls++
	return io.NopCloser(bytes.NewReader([]byte("content"))), nil
}

func (s *storageStub) Delete(_ context.Context, key string) error {
	s.deleteKeys = append(s.deleteKeys, key)
	return nil
}

func multipartHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	return request.MultipartForm.File["files"][0]
}

func newMockService(t *testing.T) (*Service, sqlmock.Sqlmock, *storageStub) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	storage := &storageStub{}
	return NewService(db, storage), mock, storage
}

func allowQuestionUpload(mock sqlmock.Sqlmock, userID, questionID int64, count int, total int64) {
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM questions`).
		WithArgs(userID, questionID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT COUNT\(\*\),COALESCE\(SUM\(size_bytes\),0\) FROM attachments WHERE question_id=\$1`).
		WithArgs(questionID).
		WillReturnRows(sqlmock.NewRows([]string{"count", "total"}).AddRow(count, total))
}

func TestUploadRejectsUnsupportedDetectedContentType(t *testing.T) {
	service, mock, storage := newMockService(t)
	allowQuestionUpload(mock, 7, 11, 0, 0)
	header := multipartHeader(t, "fake.pdf", []byte("plain text, not a PDF"))

	_, err := service.Upload(context.Background(), 7, QuestionParent, 11, []*multipart.FileHeader{header})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Upload() error = %v, want ErrValidation", err)
	}
	if len(storage.putKeys) != 0 {
		t.Fatal("unsupported content must not be stored")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUploadEnforcesFileCountAndTotalSize(t *testing.T) {
	tests := []struct {
		name  string
		count int
		total int64
		size  int64
	}{
		{name: "sixth file", count: MaxFilesPerParent, total: 1024, size: 1024},
		{name: "over total size", count: 2, total: MaxTotalBytes, size: 1},
		{name: "over single file size", count: 0, total: 0, size: MaxFileBytes + 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mock, storage := newMockService(t)
			allowQuestionUpload(mock, 7, 11, tt.count, tt.total)
			header := &multipart.FileHeader{Filename: "document.pdf", Size: tt.size}

			_, err := service.Upload(context.Background(), 7, QuestionParent, 11, []*multipart.FileHeader{header})

			if !errors.Is(err, ErrValidation) {
				t.Fatalf("Upload() error = %v, want ErrValidation", err)
			}
			if len(storage.putKeys) != 0 {
				t.Fatal("invalid attachment must not be stored")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUploadDeletesStoredObjectWhenDatabaseInsertFails(t *testing.T) {
	service, mock, storage := newMockService(t)
	allowQuestionUpload(mock, 7, 11, 0, 0)
	mock.ExpectQuery(`INSERT INTO attachments`).
		WithArgs(int64(7), int64(11), "image.png", sqlmock.AnyArg(), "image/png", int64(8)).
		WillReturnError(errors.New("database unavailable"))
	header := multipartHeader(t, "image.png", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})

	_, err := service.Upload(context.Background(), 7, QuestionParent, 11, []*multipart.FileHeader{header})

	if err == nil {
		t.Fatal("Upload() error = nil, want database error")
	}
	if len(storage.putKeys) != 1 || len(storage.deleteKeys) != 1 || storage.putKeys[0] != storage.deleteKeys[0] {
		t.Fatalf("stored object was not cleaned up: put=%v delete=%v", storage.putKeys, storage.deleteKeys)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadDoesNotReadStorageWhenViewerIsForbidden(t *testing.T) {
	service, mock, storage := newMockService(t)
	createdAt := time.Now()
	mock.ExpectQuery(`SELECT id,original_name,storage_key`).
		WithArgs(int64(30)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "original_name", "storage_key", "content_type", "size_bytes", "created_at",
			"school_post_id", "question_id", "answer_id",
		}).AddRow(30, "private.pdf", "attachments/private.pdf", "application/pdf", 100, createdAt, nil, 11, nil))
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM questions`).
		WithArgs(int64(99), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	_, body, err := service.Download(context.Background(), 99, 30)

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Download() error = %v, want ErrForbidden", err)
	}
	if body != nil || storage.getCalls != 0 {
		t.Fatal("forbidden download must not access object storage")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
