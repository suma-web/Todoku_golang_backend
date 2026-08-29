package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"twitter_golang_backend/internal/auth"
)

type Handler struct {
	repository    *Repository
	sessionSecret string
}

func NewHandler(repository *Repository, sessionSecret string) *Handler {
	return &Handler{repository: repository, sessionSecret: sessionSecret}
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request LoginRequest

	if err := decoder.Decode(&request); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"INVALID_JSON",
			"リクエストの形式が正しくありません",
		)
		return
	}

	email := strings.ToLower(strings.TrimSpace(request.Email))

	if message := validateLogin(email, request.Password); message != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", message)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	foundUser, err := h.repository.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeInvalidCredentials(w)
			return
		}

		writeError(
			w,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"サーバーエラーが発生しました",
		)
		return
	}
	if !foundUser.IsActive {
		writeError(w, http.StatusForbidden, "ACCOUNT_DISABLED", "このアカウントは無効です")
		return
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(foundUser.PasswordHash),
		[]byte(request.Password),
	); err != nil {
		writeInvalidCredentials(w)
		return
	}

	auth.SetSessionCookie(w, foundUser.ID, h.sessionSecret)

	response := LoginResponse{
		ID:        foundUser.ID,
		Name:      foundUser.Name,
		Email:     foundUser.Email,
		Birthday:  foundUser.Birthday.Format("2006-01-02"),
		CreatedAt: foundUser.CreatedAt.Format(time.RFC3339),
		Role:      foundUser.Role,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.repository.Delete(ctx, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "ユーザーが見つかりません")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "退会できませんでした")
		return
	}
	auth.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	foundUser, err := h.repository.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "ユーザーが見つかりません")
			return
		}

		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ユーザー情報を取得できませんでした")
		return
	}

	writeJSON(w, http.StatusOK, CurrentUserResponse{
		ID: foundUser.ID, Name: foundUser.Name, Bio: foundUser.Bio,
		Location: foundUser.Location, Website: foundUser.Website,
		CreatedAt: foundUser.CreatedAt.Format(time.RFC3339),
		Role:      foundUser.Role, IsActive: foundUser.IsActive,
	})
}

func (h *Handler) AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var request AdminCreateUserRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil {
		writeError(w, 400, "INVALID_JSON", "入力が不正です")
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if request.Name == "" || len(request.Password) < 8 || (request.Role != "student" && request.Role != "teacher" && request.Role != "admin") {
		writeError(w, 400, "VALIDATION_ERROR", "名前、8文字以上のパスワード、正しいRoleを指定してください")
		return
	}
	if _, err := mail.ParseAddress(request.Email); err != nil {
		writeError(w, 400, "VALIDATION_ERROR", "メールアドレスが不正です")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "ユーザーを作成できませんでした")
		return
	}
	created, err := h.repository.CreateSchoolUser(r.Context(), request.Name, request.Email, string(hash), request.Role)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, 409, "USER_ALREADY_EXISTS", "登録済みのユーザー名またはメールアドレスです")
			return
		}
		writeError(w, 500, "INTERNAL_ERROR", "ユーザーを作成できませんでした")
		return
	}
	writeJSON(w, http.StatusCreated, CurrentUserResponse{ID: created.ID, Name: created.Name, Email: created.Email, Role: created.Role, IsActive: created.IsActive, CreatedAt: created.CreatedAt.Format(time.RFC3339)})
}

func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	found, err := h.repository.FindByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "ユーザーが見つかりません")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "プロフィールを取得できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, CurrentUserResponse{
		ID: found.ID, Name: found.Name, Bio: found.Bio, Location: found.Location,
		Website: found.Website, CreatedAt: found.CreatedAt.Format(time.RFC3339),
	})
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var request UpdateProfileRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "リクエストの形式が正しくありません")
		return
	}
	name, bio := strings.TrimSpace(request.Name), strings.TrimSpace(request.Bio)
	location, website := strings.TrimSpace(request.Location), strings.TrimSpace(request.Website)
	if name == "" || len([]rune(name)) > 50 || len([]rune(bio)) > 160 || len([]rune(location)) > 30 || len([]rune(website)) > 200 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "入力文字数を確認してください")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	updated, err := h.repository.UpdateProfile(ctx, userID, name, bio, location, website)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "プロフィールを更新できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, CurrentUserResponse{
		ID: updated.ID, Name: updated.Name, Bio: updated.Bio, Location: updated.Location,
		Website: updated.Website, CreatedAt: updated.CreatedAt.Format(time.RFC3339),
	})
}

func validateLogin(email, password string) string {
	address, err := mail.ParseAddress(email)
	if email == "" || err != nil || address.Address != email {
		return "正しいメールアドレスを入力してください"
	}

	if password == "" {
		return "パスワードを入力してください"
	}

	return ""
}

func isUniqueViolation(err error) bool {
	var pgError *pgconn.PgError

	if errors.As(err, &pgError) {
		return pgError.Code == "23505"
	}

	return false
}

func writeInvalidCredentials(w http.ResponseWriter) {
	writeError(
		w,
		http.StatusUnauthorized,
		"INVALID_CREDENTIALS",
		"ユーザー名またはパスワードが違います",
	)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}
