package attachment

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"todoku_golang_backend/internal/auth"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) UploadSchoolPost(w http.ResponseWriter, r *http.Request) {
	h.upload(w, r, SchoolPostParent)
}
func (h *Handler) UploadQuestion(w http.ResponseWriter, r *http.Request) {
	h.upload(w, r, QuestionParent)
}
func (h *Handler) UploadAnswer(w http.ResponseWriter, r *http.Request) { h.upload(w, r, AnswerParent) }
func (h *Handler) ListSchoolPost(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, SchoolPostParent)
}
func (h *Handler) ListQuestion(w http.ResponseWriter, r *http.Request) { h.list(w, r, QuestionParent) }
func (h *Handler) ListAnswer(w http.ResponseWriter, r *http.Request)   { h.list(w, r, AnswerParent) }

func (h *Handler) upload(w http.ResponseWriter, r *http.Request, parent ParentType) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxTotalBytes+(1<<20))
	if err := r.ParseMultipartForm(MaxTotalBytes); err != nil {
		writeError(w, http.StatusBadRequest, "添付ファイルが大きすぎます")
		return
	}
	parentID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "IDが不正です")
		return
	}
	items, err := h.service.Upload(r.Context(), currentUserID(r), parent, parentID, r.MultipartForm.File["files"])
	switch {
	case errors.Is(err, ErrValidation):
		writeError(w, http.StatusBadRequest, "PDF・JPEG・PNG・WebPを最大5件、1件10MB・合計25MB以内で指定してください")
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "添付する権限がありません")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "ファイルを保存できませんでした")
	default:
		writeJSON(w, http.StatusCreated, items)
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, parent ParentType) {
	parentID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "IDが不正です")
		return
	}
	items, err := h.service.List(r.Context(), currentUserID(r), parent, parentID)
	if errors.Is(err, ErrForbidden) {
		writeError(w, http.StatusForbidden, "添付ファイルを閲覧する権限がありません")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "添付ファイルを取得できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "IDが不正です")
		return
	}
	item, body, err := h.service.Download(r.Context(), currentUserID(r), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "添付ファイルが見つかりません")
		return
	}
	if errors.Is(err, ErrForbidden) {
		writeError(w, http.StatusForbidden, "添付ファイルを閲覧する権限がありません")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "添付ファイルを取得できませんでした")
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", item.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(item.SizeBytes, 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.OriginalName}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, body)
}

func currentUserID(r *http.Request) int64 { id, _ := auth.UserID(r.Context()); return id }

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
