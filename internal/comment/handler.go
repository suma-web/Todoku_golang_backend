package comment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"twitter_golang_backend/internal/auth"
)

type Handler struct {
	repository *Repository
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{repository: repository}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	postID, ok := parsePositiveID(chi.URLParam(r, "id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "投稿IDが不正です")
		return
	}
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request CreateRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "リクエストの形式が正しくありません")
		return
	}
	text := strings.TrimSpace(request.Comment)
	if text == "" {
		writeError(w, http.StatusBadRequest, "COMMENT_REQUIRED", "コメントを入力してください")
		return
	}
	if utf8.RuneCountInString(text) > 140 {
		writeError(w, http.StatusBadRequest, "COMMENT_TOO_LONG", "コメントは140文字以内で入力してください")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	exists, err := h.repository.PostExists(ctx, postID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "コメントを保存できませんでした")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "投稿が見つかりません")
		return
	}
	created, err := h.repository.Create(ctx, postID, userID, text)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "コメントを保存できませんでした")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	postID, ok := parsePositiveID(chi.URLParam(r, "id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "投稿IDが不正です")
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
	exists, err := h.repository.PostExists(ctx, postID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "コメント一覧を取得できませんでした")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "投稿が見つかりません")
		return
	}
	comments, err := h.repository.ListByPostID(ctx, postID, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "コメント一覧を取得できませんでした")
		return
	}
	hasMore := len(comments) > limit
	if hasMore {
		comments = comments[:limit]
	}
	writeJSON(w, http.StatusOK, ListResponse{
		Comments: comments, Limit: limit, Offset: offset, HasMore: hasMore,
	})
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
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
	comments, err := h.repository.ListByUserID(ctx, userID, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "コメント一覧を取得できませんでした")
		return
	}
	hasMore := len(comments) > limit
	if hasMore {
		comments = comments[:limit]
	}
	writeJSON(w, http.StatusOK, ListResponse{
		Comments: comments, Limit: limit, Offset: offset, HasMore: hasMore,
	})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	commentID, ok := parsePositiveID(chi.URLParam(r, "id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_COMMENT_ID", "コメントIDが不正です")
		return
	}
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.repository.Delete(ctx, commentID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "COMMENT_NOT_FOUND", "コメントが見つからないか、削除する権限がありません")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "コメントを削除できませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parsePositiveID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
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

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
