package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
