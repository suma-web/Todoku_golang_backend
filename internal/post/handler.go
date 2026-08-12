package post

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"twitter_golang_backend/internal/auth"
)

type Handler struct {
	repository *Repository
	uploadDir  string
}

func NewHandler(repository *Repository, uploadDir string) *Handler {
	return &Handler{repository: repository, uploadDir: uploadDir}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_FORM", "投稿データが不正、または画像が大きすぎます")
		return
	}
	doc := strings.TrimSpace(r.FormValue("doc"))
	if utf8.RuneCountInString(doc) > 140 {
		writeError(w, http.StatusBadRequest, "DOC_TOO_LONG", "投稿内容は140文字以内にしてください")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		writeError(w, http.StatusBadRequest, "INVALID_IMAGE", "画像を読み込めません")
		return
	}
	if file != nil {
		defer file.Close()
	}
	if doc == "" && file == nil {
		writeError(w, http.StatusBadRequest, "CONTENT_REQUIRED", "投稿内容または画像を指定してください")
		return
	}

	var imageURL *string
	if file != nil {
		url, err := h.saveImage(file, header)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_IMAGE", err.Error())
			return
		}
		imageURL = &url
	}

	userID, _ := auth.UserID(r.Context())
	ctx := r.Context()
	created, err := h.repository.Create(ctx, userID, doc, imageURL)
	if err != nil {
		if imageURL != nil {
			_ = os.Remove(filepath.Join(h.uploadDir, filepath.Base(*imageURL)))
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "投稿を保存できませんでした")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit, err := parseNonNegativeQuery(r, "limit", 20)
	if err != nil || limit < 1 || limit > 100 {
		writeError(w, http.StatusBadRequest, "INVALID_LIMIT", "limitは1以上100以下の整数で指定してください")
		return
	}
	offset, err := parseNonNegativeQuery(r, "offset", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OFFSET", "offsetは0以上の整数で指定してください")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	userID, _ := auth.UserID(r.Context())

	var posts []Post
	name := strings.TrimSpace(r.URL.Query().Get("user"))
	if name == "" {
		posts, err = h.repository.List(ctx, userID, limit+1, offset)
	} else {
		posts, err = h.repository.ListByUserName(ctx, userID, name, limit+1, offset)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "投稿一覧を取得できませんでした")
		return
	}

	hasMore := len(posts) > limit
	if hasMore {
		posts = posts[:limit]
	}
	if name != "" {
		writeJSON(w, http.StatusOK, UserTweetsResponse{
			Tweets: posts, Limit: limit, Offset: offset, HasMore: hasMore,
		})
		return
	}

	writeJSON(w, http.StatusOK, ListResponse{
		Posts: posts, Limit: limit, Offset: offset, HasMore: hasMore,
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "投稿IDが不正です")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	userID, _ := auth.UserID(r.Context())

	found, err := h.repository.FindByID(ctx, postID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "投稿が見つかりません")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "投稿を取得できませんでした")
		return
	}

	writeJSON(w, http.StatusOK, found)
}

func (h *Handler) Retweet(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "投稿IDが不正です")
		return
	}
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	exists, err := h.repository.PostExists(ctx, postID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "リツイートできませんでした")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "投稿が見つかりません")
		return
	}

	response, err := h.repository.Retweet(ctx, postID, userID)
	if err != nil {
		if errors.Is(err, ErrAlreadyRetweeted) {
			writeError(w, http.StatusConflict, "ALREADY_RETWEETED", "この投稿はリツイート済みです")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "リツイートできませんでした")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) UndoRetweet(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "投稿IDが不正です")
		return
	}
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	response, err := h.repository.UndoRetweet(ctx, postID, userID)
	if err != nil {
		if errors.Is(err, ErrNotRetweeted) {
			writeError(w, http.StatusNotFound, "RETWEET_NOT_FOUND", "この投稿はリツイートされていません")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "リツイートを解除できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ListMyRetweets(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}
	limit, err := parseNonNegativeQuery(r, "limit", 20)
	if err != nil || limit < 1 || limit > 100 {
		writeError(w, http.StatusBadRequest, "INVALID_LIMIT", "limitは1以上100以下の整数で指定してください")
		return
	}
	offset, err := parseNonNegativeQuery(r, "offset", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OFFSET", "offsetは0以上の整数で指定してください")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	posts, err := h.repository.ListRetweetedByUser(ctx, userID, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "リツイート一覧を取得できませんでした")
		return
	}
	hasMore := len(posts) > limit
	if hasMore {
		posts = posts[:limit]
	}
	writeJSON(w, http.StatusOK, ListResponse{
		Posts: posts, Limit: limit, Offset: offset, HasMore: hasMore,
	})
}

func (h *Handler) Like(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "投稿IDが不正です")
		return
	}
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	exists, err := h.repository.PostExists(ctx, postID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "いいねできませんでした")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "投稿が見つかりません")
		return
	}

	response, err := h.repository.Like(ctx, postID, userID)
	if err != nil {
		if errors.Is(err, ErrAlreadyLiked) {
			writeError(w, http.StatusConflict, "ALREADY_LIKED", "この投稿はいいね済みです")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "いいねできませんでした")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) UndoLike(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "投稿IDが不正です")
		return
	}
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	response, err := h.repository.UndoLike(ctx, postID, userID)
	if err != nil {
		if errors.Is(err, ErrNotLiked) {
			writeError(w, http.StatusNotFound, "LIKE_NOT_FOUND", "この投稿はいいねされていません")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "いいねを解除できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) Bookmark(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "投稿IDが不正です")
		return
	}
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	exists, err := h.repository.PostExists(ctx, postID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ブックマークできませんでした")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "投稿が見つかりません")
		return
	}
	response, err := h.repository.Bookmark(ctx, postID, userID)
	if err != nil {
		if errors.Is(err, ErrAlreadyBookmarked) {
			writeError(w, http.StatusConflict, "ALREADY_BOOKMARKED", "この投稿はブックマーク済みです")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ブックマークできませんでした")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) UndoBookmark(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "投稿IDが不正です")
		return
	}
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	response, err := h.repository.UndoBookmark(ctx, postID, userID)
	if err != nil {
		if errors.Is(err, ErrNotBookmarked) {
			writeError(w, http.StatusNotFound, "BOOKMARK_NOT_FOUND", "この投稿はブックマークされていません")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ブックマークを解除できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ListBookmarks(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}
	limit, err := parseNonNegativeQuery(r, "limit", 20)
	if err != nil || limit < 1 || limit > 100 {
		writeError(w, http.StatusBadRequest, "INVALID_LIMIT", "limitは1以上100以下の整数で指定してください")
		return
	}
	offset, err := parseNonNegativeQuery(r, "offset", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OFFSET", "offsetは0以上の整数で指定してください")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	posts, err := h.repository.ListBookmarkedByUser(ctx, userID, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ブックマーク一覧を取得できませんでした")
		return
	}
	hasMore := len(posts) > limit
	if hasMore {
		posts = posts[:limit]
	}
	writeJSON(w, http.StatusOK, ListResponse{Posts: posts, Limit: limit, Offset: offset, HasMore: hasMore})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "投稿IDが不正です")
		return
	}
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	imageURL, err := h.repository.Delete(ctx, postID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "投稿が見つからないか、削除する権限がありません")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "投稿を削除できませんでした")
		return
	}
	if imageURL != nil {
		_ = os.Remove(filepath.Join(h.uploadDir, filepath.Base(*imageURL)))
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseNonNegativeQuery(r *http.Request, key string, fallback int) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return parsed, nil
}

func (h *Handler) saveImage(file multipart.File, header *multipart.FileHeader) (string, error) {
	if header.Size > 5<<20 {
		return "", fmt.Errorf("画像は5MB以下にしてください")
	}
	first := make([]byte, 512)
	n, err := file.Read(first)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("画像を読み込めません")
	}
	contentType := http.DetectContentType(first[:n])
	extensions := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp"}
	ext, ok := extensions[contentType]
	if !ok {
		return "", fmt.Errorf("JPEG、PNG、GIF、WebP画像を選択してください")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("画像を読み込めません")
	}
	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		return "", fmt.Errorf("画像を保存できません")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("画像を保存できません")
	}
	name := hex.EncodeToString(random) + ext
	destination, err := os.OpenFile(filepath.Join(h.uploadDir, name), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return "", fmt.Errorf("画像を保存できません")
	}
	defer destination.Close()
	if _, err := io.Copy(destination, file); err != nil {
		return "", fmt.Errorf("画像を保存できません")
	}
	return "/uploads/" + name, nil
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
