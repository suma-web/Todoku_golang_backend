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

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request SignupRequest

	if err := decoder.Decode(&request); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"INVALID_JSON",
			"リクエストの形式が正しくありません",
		)
		return
	}

	name := strings.TrimSpace(request.Name)
	email := strings.ToLower(strings.TrimSpace(request.Email))

	if message := validateSignup(request, name, email); message != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", message)
		return
	}

	birthday, err := time.Parse("2006-01-02", request.Birthday)
	if err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"INVALID_BIRTHDAY",
			"生年月日の形式が正しくありません",
		)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(request.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"サーバーエラーが発生しました",
		)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	createdUser, err := h.repository.Create(
		ctx,
		name,
		email,
		birthday,
		string(passwordHash),
	)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(
				w,
				http.StatusConflict,
				"EMAIL_ALREADY_EXISTS",
				"このメールアドレスは既に登録されています",
			)
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

	response := SignupResponse{
		ID:        createdUser.ID,
		Name:      createdUser.Name,
		Email:     createdUser.Email,
		Birthday:  createdUser.Birthday.Format("2006-01-02"),
		CreatedAt: createdUser.CreatedAt.Format(time.RFC3339),
	}

	writeJSON(w, http.StatusCreated, response)
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

	identifier := normalizeLoginIdentifier(request)

	if message := validateLogin(identifier, request.Password); message != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", message)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	foundUser, err := h.repository.FindByLoginIdentifier(ctx, identifier)
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
	})
}

func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	viewerID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}
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
	followedByMe := false
	if viewerID != found.ID {
		followedByMe, err = h.repository.IsFollowing(ctx, viewerID, found.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "プロフィールを取得できませんでした")
			return
		}
	}
	writeJSON(w, http.StatusOK, CurrentUserResponse{
		ID: found.ID, Name: found.Name, Bio: found.Bio, Location: found.Location,
		Website: found.Website, CreatedAt: found.CreatedAt.Format(time.RFC3339),
		FollowedByMe: followedByMe,
	})
}

func (h *Handler) Follow(w http.ResponseWriter, r *http.Request) {
	h.changeFollow(w, r, true)
}

func (h *Handler) Unfollow(w http.ResponseWriter, r *http.Request) {
	h.changeFollow(w, r, false)
}

func (h *Handler) changeFollow(w http.ResponseWriter, r *http.Request, follow bool) {
	viewerID, ok := auth.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインが必要です")
		return
	}
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	target, err := h.repository.FindByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "ユーザーが見つかりません")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "フォロー状態を変更できませんでした")
		return
	}
	if viewerID == target.ID {
		writeError(w, http.StatusBadRequest, "CANNOT_FOLLOW_SELF", "自分自身はフォローできません")
		return
	}
	if follow {
		err = h.repository.Follow(ctx, viewerID, target.ID)
	} else {
		err = h.repository.Unfollow(ctx, viewerID, target.ID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "フォロー状態を変更できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, FollowResponse{FollowedByMe: follow})
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

func validateSignup(request SignupRequest, name, email string) string {
	if name == "" {
		return "名前を入力してください"
	}

	if len([]rune(name)) > 50 {
		return "名前は50文字以内で入力してください"
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return "正しいメールアドレスを入力してください"
	}

	if len(request.Password) < 8 {
		return "パスワードは8文字以上で入力してください"
	}

	if request.Birthday == "" {
		return "生年月日を入力してください"
	}

	return ""
}

func normalizeLoginIdentifier(request LoginRequest) string {
	if strings.TrimSpace(request.Identifier) != "" {
		return strings.TrimSpace(request.Identifier)
	}

	if strings.TrimSpace(request.Email) != "" {
		return strings.ToLower(strings.TrimSpace(request.Email))
	}

	return strings.TrimSpace(request.Name)
}

func validateLogin(identifier, password string) string {
	if identifier == "" {
		return "ユーザー名またはメールアドレスを入力してください"
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
