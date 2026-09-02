package schooladmin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func send(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func adminError(w http.ResponseWriter, status int, message string) {
	send(w, status, map[string]any{"error": map[string]string{"message": message}})
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListUsers(r.Context())
	if err != nil {
		adminError(w, http.StatusInternalServerError, "ユーザー一覧を取得できませんでした")
		return
	}
	send(w, http.StatusOK, items)
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		adminError(w, http.StatusBadRequest, "ユーザーIDが不正です")
		return
	}
	var input User
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		adminError(w, http.StatusBadRequest, "入力が不正です")
		return
	}
	item, err := h.service.UpdateUser(r.Context(), id, input.Role, input.IsActive)
	switch {
	case errors.Is(err, ErrValidation):
		adminError(w, http.StatusBadRequest, "Roleが不正です")
	case errors.Is(err, ErrNotFound):
		adminError(w, http.StatusNotFound, "ユーザーが見つかりません")
	case err != nil:
		adminError(w, http.StatusInternalServerError, "ユーザーを更新できませんでした")
	default:
		send(w, http.StatusOK, item)
	}
}
