package schoolgroup

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

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func fail(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]any{"error": map[string]string{"message": message}})
}

func idParam(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, key), 10, 64)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, "所属一覧を取得できませんでした")
		return
	}
	respond(w, http.StatusOK, items)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input Group
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		fail(w, http.StatusBadRequest, "入力が不正です")
		return
	}
	item, err := h.service.Create(r.Context(), input)
	switch {
	case errors.Is(err, ErrValidation):
		fail(w, http.StatusBadRequest, "所属名または種類が不正です")
	case errors.Is(err, ErrConflict):
		fail(w, http.StatusConflict, "同じ所属が既に存在します")
	case err != nil:
		fail(w, http.StatusInternalServerError, "所属を作成できませんでした")
	default:
		respond(w, http.StatusCreated, item)
	}
}

func (h *Handler) UserGroups(w http.ResponseWriter, r *http.Request) {
	targetID, err := idParam(r, "userId")
	if err != nil {
		fail(w, http.StatusBadRequest, "ユーザーIDが不正です")
		return
	}
	viewerID, _ := auth.UserID(r.Context())
	items, err := h.service.UserGroups(r.Context(), viewerID, targetID)
	switch {
	case errors.Is(err, ErrForbidden):
		fail(w, http.StatusForbidden, "このユーザーの所属を閲覧する権限がありません")
	case errors.Is(err, ErrValidation):
		fail(w, http.StatusBadRequest, "ユーザーIDが不正です")
	case err != nil:
		fail(w, http.StatusInternalServerError, "所属を取得できませんでした")
	default:
		respond(w, http.StatusOK, items)
	}
}

func (h *Handler) Members(w http.ResponseWriter, r *http.Request) {
	groupID, err := idParam(r, "groupId")
	if err != nil {
		fail(w, http.StatusBadRequest, "所属IDが不正です")
		return
	}
	items, err := h.service.Members(r.Context(), groupID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "所属メンバーを取得できませんでした")
		return
	}
	respond(w, http.StatusOK, items)
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	groupID, err := idParam(r, "groupId")
	var input struct {
		UserID int64 `json:"user_id"`
	}
	if err != nil || json.NewDecoder(r.Body).Decode(&input) != nil {
		fail(w, http.StatusBadRequest, "ユーザーIDまたは所属IDが不正です")
		return
	}
	err = h.service.AddMember(r.Context(), groupID, input.UserID)
	switch {
	case errors.Is(err, ErrValidation):
		fail(w, http.StatusBadRequest, "ユーザーIDまたは所属IDが不正です")
	case errors.Is(err, ErrNotFound):
		fail(w, http.StatusNotFound, "ユーザーまたは所属が見つかりません")
	case err != nil:
		fail(w, http.StatusInternalServerError, "所属を追加できませんでした")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	groupID, groupErr := idParam(r, "groupId")
	userID, userErr := idParam(r, "userId")
	if groupErr != nil || userErr != nil {
		fail(w, http.StatusBadRequest, "IDが不正です")
		return
	}
	err := h.service.RemoveMember(r.Context(), groupID, userID)
	switch {
	case errors.Is(err, ErrNotFound):
		fail(w, http.StatusNotFound, "所属情報が見つかりません")
	case err != nil:
		fail(w, http.StatusInternalServerError, "所属を削除できませんでした")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	groupID, err := idParam(r, "groupId")
	if err != nil {
		fail(w, http.StatusBadRequest, "所属IDが不正です")
		return
	}
	err = h.service.Delete(r.Context(), groupID)
	switch {
	case errors.Is(err, ErrNotFound):
		fail(w, http.StatusNotFound, "所属が見つかりません")
	case errors.Is(err, ErrConflict):
		fail(w, http.StatusConflict, "使用中の所属は削除できません")
	case err != nil:
		fail(w, http.StatusInternalServerError, "所属を削除できませんでした")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
