package question

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

func output(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func failure(w http.ResponseWriter, status int, code, message string) {
	output(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func userID(r *http.Request) int64 {
	id, _ := auth.UserID(r.Context())
	return id
}

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListCategories(r.Context())
	if err != nil {
		failure(w, http.StatusInternalServerError, "INTERNAL_ERROR", "質問カテゴリを取得できませんでした")
		return
	}
	output(w, http.StatusOK, items)
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var input Category
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		failure(w, http.StatusBadRequest, "INVALID_JSON", "入力が不正です")
		return
	}
	item, err := h.service.CreateCategory(r.Context(), input)
	if errors.Is(err, ErrValidation) {
		failure(w, http.StatusBadRequest, "VALIDATION_ERROR", "カテゴリ名と担当部署を指定してください")
		return
	}
	if err != nil {
		failure(w, http.StatusConflict, "CATEGORY_EXISTS", "カテゴリを作成できませんでした")
		return
	}
	output(w, http.StatusCreated, item)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input Question
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		failure(w, http.StatusBadRequest, "INVALID_JSON", "入力が不正です")
		return
	}
	item, err := h.service.Create(r.Context(), userID(r), input)
	if err != nil {
		code := "VALIDATION_ERROR"
		message := "カテゴリ、タイトル、本文、公開範囲を確認してください"
		if !errors.Is(err, ErrValidation) {
			code, message = "INTERNAL_ERROR", "質問を作成できませんでした"
		}
		failure(w, http.StatusBadRequest, code, message)
		return
	}
	output(w, http.StatusCreated, item)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context(), userID(r), r.URL.Query().Get("status"))
	if err != nil {
		failure(w, http.StatusInternalServerError, "INTERNAL_ERROR", "質問一覧を取得できませんでした")
		return
	}
	output(w, http.StatusOK, items)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		failure(w, http.StatusBadRequest, "INVALID_ID", "質問IDが不正です")
		return
	}
	item, err := h.service.Get(r.Context(), id, userID(r))
	if errors.Is(err, ErrNotFound) {
		failure(w, http.StatusNotFound, "QUESTION_NOT_FOUND", "質問が見つかりません")
		return
	}
	if err != nil {
		failure(w, http.StatusInternalServerError, "INTERNAL_ERROR", "回答を取得できませんでした")
		return
	}
	output(w, http.StatusOK, item)
}

func (h *Handler) ListAnswers(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		failure(w, http.StatusBadRequest, "INVALID_ID", "質問IDが不正です")
		return
	}
	items, err := h.service.ListAnswers(r.Context(), id, userID(r))
	if errors.Is(err, ErrNotFound) {
		failure(w, http.StatusNotFound, "QUESTION_NOT_FOUND", "質問が見つかりません")
		return
	}
	if err != nil {
		failure(w, http.StatusInternalServerError, "INTERNAL_ERROR", "回答を取得できませんでした")
		return
	}
	output(w, http.StatusOK, items)
}

func (h *Handler) Answer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		failure(w, http.StatusBadRequest, "INVALID_ID", "質問IDが不正です")
		return
	}
	var input Answer
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		failure(w, http.StatusBadRequest, "VALIDATION_ERROR", "回答本文を入力してください")
		return
	}
	item, err := h.service.Answer(r.Context(), id, userID(r), input.Content)
	switch {
	case errors.Is(err, ErrForbidden):
		failure(w, http.StatusForbidden, "FORBIDDEN", "この質問へ回答する権限がありません")
	case errors.Is(err, ErrValidation):
		failure(w, http.StatusBadRequest, "VALIDATION_ERROR", "回答本文を入力してください")
	case err != nil:
		failure(w, http.StatusInternalServerError, "INTERNAL_ERROR", "回答できませんでした")
	default:
		output(w, http.StatusCreated, item)
	}
}

func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		failure(w, http.StatusBadRequest, "INVALID_ID", "質問IDが不正です")
		return
	}
	err = h.service.Resolve(r.Context(), id, userID(r))
	if errors.Is(err, ErrForbidden) {
		failure(w, http.StatusForbidden, "FORBIDDEN", "質問者本人だけが解決済みにできます")
		return
	}
	if err != nil {
		failure(w, http.StatusInternalServerError, "INTERNAL_ERROR", "解決済みにできませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
