package schoolpost

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"twitter_golang_backend/internal/auth"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func out(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func bad(w http.ResponseWriter, status int, message string) {
	out(w, status, map[string]any{"error": map[string]string{"message": message}})
}

func postID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func currentUserID(r *http.Request) int64 {
	id, _ := auth.UserID(r.Context())
	return id
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input Post
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		bad(w, http.StatusBadRequest, "入力が不正です")
		return
	}
	item, err := h.service.Create(r.Context(), currentUserID(r), input)
	switch {
	case errors.Is(err, ErrValidation):
		bad(w, http.StatusBadRequest, "タイトル、本文、対象所属は必須です")
	case err != nil:
		bad(w, http.StatusBadRequest, "投稿内容が不正です")
	default:
		out(w, http.StatusCreated, item)
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := postID(r)
	if err != nil {
		bad(w, http.StatusBadRequest, "IDが不正です")
		return
	}
	item, err := h.service.Get(r.Context(), id, currentUserID(r))
	if err != nil {
		bad(w, http.StatusNotFound, "投稿が見つかりません")
		return
	}
	out(w, http.StatusOK, item)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := postID(r)
	if err != nil {
		bad(w, http.StatusBadRequest, "IDが不正です")
		return
	}
	err = h.service.Delete(r.Context(), id, currentUserID(r))
	if errors.Is(err, ErrNotFound) {
		bad(w, http.StatusNotFound, "投稿が見つからないか権限がありません")
		return
	}
	if err != nil {
		bad(w, http.StatusInternalServerError, "削除できませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Timeline(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Timeline(r.Context(), currentUserID(r))
	if err != nil {
		bad(w, http.StatusInternalServerError, "タイムラインを取得できませんでした")
		return
	}
	out(w, http.StatusOK, items)
}

func (h *Handler) Read(w http.ResponseWriter, r *http.Request) {
	id, err := postID(r)
	if err != nil {
		bad(w, http.StatusBadRequest, "IDが不正です")
		return
	}
	err = h.service.MarkRead(r.Context(), id, currentUserID(r))
	if errors.Is(err, ErrForbidden) {
		bad(w, http.StatusForbidden, "この投稿の対象者ではありません")
		return
	}
	if err != nil {
		bad(w, http.StatusInternalServerError, "既読状態を保存できませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Authored(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Authored(r.Context(), currentUserID(r))
	if err != nil {
		bad(w, http.StatusInternalServerError, "作成した連絡を取得できませんでした")
		return
	}
	out(w, http.StatusOK, items)
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	id, err := postID(r)
	if err != nil {
		bad(w, http.StatusBadRequest, "IDが不正です")
		return
	}
	item, err := h.service.Status(r.Context(), id, currentUserID(r))
	if errors.Is(err, ErrForbidden) {
		bad(w, http.StatusForbidden, "既読状況は連絡作成者または管理者だけが閲覧できます")
		return
	}
	if err != nil {
		bad(w, http.StatusInternalServerError, "既読状況を取得できませんでした")
		return
	}
	out(w, http.StatusOK, item)
}
