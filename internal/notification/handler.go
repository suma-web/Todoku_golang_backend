package notification

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
	"todoku_golang_backend/internal/auth"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, message string) {
	write(w, status, map[string]any{"error": map[string]string{"message": message}})
}
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		fail(w, 401, "ログインしてください")
		return
	}
	result, err := h.service.List(r.Context(), uid)
	if err != nil {
		fail(w, 500, "通知を取得できませんでした")
		return
	}
	write(w, 200, result)
}
func (h *Handler) Read(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		fail(w, 401, "ログインしてください")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		fail(w, 400, "IDが不正です")
		return
	}
	err = h.service.MarkRead(r.Context(), id, uid)
	if errors.Is(err, ErrNotFound) {
		fail(w, 404, "通知が見つかりません")
		return
	}
	if err != nil {
		fail(w, 500, "通知を既読にできませんでした")
		return
	}
	w.WriteHeader(204)
}
