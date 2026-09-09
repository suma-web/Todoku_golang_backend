package schooladmin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"todoku_golang_backend/internal/auth"
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
	actorID, ok := auth.UserID(r.Context())
	if !ok {
		adminError(w, http.StatusUnauthorized, "ログインが必要です")
		return
	}
	item, err := h.service.UpdateUser(r.Context(), actorID, id, input.Role, input.IsActive)
	switch {
	case errors.Is(err, ErrValidation):
		adminError(w, http.StatusBadRequest, "Roleが不正です")
	case errors.Is(err, ErrNotFound):
		adminError(w, http.StatusNotFound, "ユーザーが見つかりません")
	case errors.Is(err, ErrSelfMutation):
		adminError(w, http.StatusConflict, "自分自身のRoleまたは有効状態は変更できません")
	case errors.Is(err, ErrLastActiveAdmin):
		adminError(w, http.StatusConflict, "有効な管理者を最低1人残す必要があります")
	case err != nil:
		adminError(w, http.StatusInternalServerError, "ユーザーを更新できませんでした")
	default:
		send(w, http.StatusOK, item)
	}
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		adminError(w, http.StatusBadRequest, "ユーザーIDが不正です")
		return
	}
	actorID, ok := auth.UserID(r.Context())
	if !ok {
		adminError(w, http.StatusUnauthorized, "ログインが必要です")
		return
	}
	err = h.service.DeleteUser(r.Context(), actorID, id)
	switch {
	case errors.Is(err, ErrValidation):
		adminError(w, http.StatusBadRequest, "ユーザーIDが不正です")
	case errors.Is(err, ErrNotFound):
		adminError(w, http.StatusNotFound, "ユーザーが見つかりません")
	case errors.Is(err, ErrSelfMutation):
		adminError(w, http.StatusConflict, "自分自身のアカウントは削除できません")
	case errors.Is(err, ErrLastActiveAdmin):
		adminError(w, http.StatusConflict, "有効な管理者を最低1人残す必要があります")
	case err != nil:
		adminError(w, http.StatusInternalServerError, "ユーザーを削除できませんでした")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
