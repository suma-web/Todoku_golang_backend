package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const cookieName = "session"

type userIDKey struct{}
type roleKey struct{}

func SetSessionCookie(w http.ResponseWriter, userID int64, secret string, secure bool) {
	payload := strconv.FormatInt(userID, 10)
	token := payload + "." + sign(payload, secret)
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", HttpOnly: true,
		SameSite: sameSite, Secure: secure, MaxAge: int((7 * 24 * time.Hour).Seconds()),
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", HttpOnly: true,
		SameSite: sameSite, Secure: secure, MaxAge: -1, Expires: time.Unix(0, 0),
	})
}

func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				writeUnauthorized(w, "ログインが必要です")
				return
			}
			parts := strings.Split(cookie.Value, ".")
			if len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(sign(parts[0], secret))) {
				writeUnauthorized(w, "セッションが無効です")
				return
			}
			userID, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil || userID <= 0 {
				writeUnauthorized(w, "セッションが無効です")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, userID)))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"UNAUTHORIZED","message":"` + message + `"}}`))
}

func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey{}).(int64)
	return id, ok
}

func RequireRole(db *sql.DB, roles ...string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := UserID(r.Context())
			if !ok {
				writeUnauthorized(w, "ログインが必要です")
				return
			}
			var role string
			var active bool
			if err := db.QueryRowContext(r.Context(), `SELECT role,is_active FROM users WHERE id=$1`, id).Scan(&role, &active); err != nil || !active {
				writeUnauthorized(w, "アカウントが無効です")
				return
			}
			if !allowed[role] {
				http.Error(w, `{"error":{"code":"FORBIDDEN","message":"権限がありません"}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), roleKey{}, role)))
		})
	}
}

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
