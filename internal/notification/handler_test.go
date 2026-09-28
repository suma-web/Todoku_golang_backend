package notification

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"todoku_golang_backend/internal/auth"
)

type stub struct {
	userID int64
	found  bool
	err    error
}

func (s *stub) List(_ context.Context, uid int64) (List, error) {
	s.userID = uid
	return List{Items: []Notification{}, UnreadCount: 3}, s.err
}
func (s *stub) MarkRead(_ context.Context, _, uid int64) (bool, error) {
	s.userID = uid
	return s.found, s.err
}
func TestNotificationAPI(t *testing.T) {
	for _, test := range []struct {
		name, method, path string
		cookie, found      bool
		err                error
		status             int
	}{
		{"list", "GET", "/api/notifications", true, true, nil, 200},
		{"read", "POST", "/api/notifications/1/read", true, true, nil, 204},
		{"not owned", "POST", "/api/notifications/1/read", true, false, nil, 404},
		{"bad id", "POST", "/api/notifications/0/read", true, true, nil, 400},
		{"anonymous", "GET", "/api/notifications", false, true, nil, 401},
		{"database failure", "GET", "/api/notifications", true, true, errors.New("db"), 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repo := &stub{found: test.found, err: test.err}
			h := NewHandler(NewService(repo))
			router := chi.NewRouter()
			secret := strings.Repeat("s", 32)
			router.Use(auth.RequireAuth(secret, db))
			router.Get("/api/notifications", h.List)
			router.Post("/api/notifications/{id}/read", h.Read)
			request := httptest.NewRequest(test.method, test.path, nil)
			if test.cookie {
				mock.ExpectQuery(`SELECT role,is_active FROM users`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"role", "is_active"}).AddRow("student", true))
				cookie := httptest.NewRecorder()
				auth.SetSessionCookie(cookie, 7, secret, false)
				request.AddCookie(cookie.Result().Cookies()[0])
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if test.status == http.StatusOK && (!strings.Contains(recorder.Body.String(), `"unread_count":3`) || repo.userID != 7) {
				t.Fatal("list must use session user and include unread count")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
