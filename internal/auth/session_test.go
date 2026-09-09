package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSetSessionCookieForLocalDevelopment(t *testing.T) {
	recorder := httptest.NewRecorder()

	SetSessionCookie(recorder, 1, strings.Repeat("s", 32), false)

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}
	if cookies[0].Secure {
		t.Error("local development cookie must not be Secure")
	}
	if cookies[0].SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookies[0].SameSite)
	}
}

func TestRequireAuthRejectsInactiveUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT role,is_active FROM users`).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "is_active"}).AddRow("student", false))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	cookieRecorder := httptest.NewRecorder()
	SetSessionCookie(cookieRecorder, 1, strings.Repeat("s", 32), false)
	request.AddCookie(cookieRecorder.Result().Cookies()[0])

	called := false
	handler := RequireAuth(strings.Repeat("s", 32), db)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized || called {
		t.Fatalf("status = %d, called = %v", recorder.Code, called)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSetSessionCookieForCrossSiteHTTPS(t *testing.T) {
	recorder := httptest.NewRecorder()

	SetSessionCookie(recorder, 1, strings.Repeat("s", 32), true)

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}
	if !cookies[0].Secure {
		t.Error("production cookie must be Secure")
	}
	if cookies[0].SameSite != http.SameSiteNoneMode {
		t.Errorf("SameSite = %v, want None", cookies[0].SameSite)
	}
}
