package search

import (
	"encoding/json"
	"errors"
	"net/http"

	"twitter_golang_backend/internal/auth"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	response, err := h.service.Search(r.Context(), userID, r.URL.Query().Get("q"))
	if errors.Is(err, ErrQueryTooLong) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"message": "検索語は100文字以内にしてください"}})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"message": "検索できませんでした"}})
		return
	}
	writeJSON(w, http.StatusOK, response)
}
