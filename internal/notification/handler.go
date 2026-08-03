package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"twitter_golang_backend/internal/auth"
)

type Handler struct {
	repository *Repository
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{repository: repository}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "ログインが必要です")
		return
	}
	limit, err := parseQuery(r, "limit", 20)
	if err != nil || limit < 1 || limit > 100 {
		writeError(w, http.StatusBadRequest, "limitは1以上100以下で指定してください")
		return
	}
	offset, err := parseQuery(r, "offset", 0)
	if err != nil || offset < 0 {
		writeError(w, http.StatusBadRequest, "offsetは0以上で指定してください")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := h.repository.ListByRecipient(ctx, userID, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "通知一覧を取得できませんでした")
		return
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	writeJSON(w, http.StatusOK, ListResponse{
		Notifications: items, Limit: limit, Offset: offset, HasMore: hasMore,
	})
}

func parseQuery(r *http.Request, key string, fallback int) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return parsed, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"message": message},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
