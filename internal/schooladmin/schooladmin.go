package schooladmin

import (
	"database/sql"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

type User struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
}
type Handler struct{ db *sql.DB }

func NewHandler(db *sql.DB) *Handler { return &Handler{db} }
func send(w http.ResponseWriter, s int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	_ = json.NewEncoder(w).Encode(v)
}
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, e := h.db.QueryContext(r.Context(), `SELECT id,name,email,role,is_active FROM users ORDER BY role,name,id`)
	if e != nil {
		send(w, 500, map[string]any{"error": map[string]string{"message": "ユーザー一覧を取得できませんでした"}})
		return
	}
	defer rows.Close()
	items := []User{}
	for rows.Next() {
		var u User
		if rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.IsActive) != nil {
			send(w, 500, map[string]any{"error": map[string]string{"message": "ユーザー一覧を取得できませんでした"}})
			return
		}
		items = append(items, u)
	}
	send(w, 200, items)
}
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil {
		send(w, 400, map[string]any{"error": map[string]string{"message": "ユーザーIDが不正です"}})
		return
	}
	var input User
	if json.NewDecoder(r.Body).Decode(&input) != nil || (input.Role != "student" && input.Role != "teacher" && input.Role != "admin") {
		send(w, 400, map[string]any{"error": map[string]string{"message": "Roleが不正です"}})
		return
	}
	e = h.db.QueryRowContext(r.Context(), `UPDATE users SET role=$2,is_active=$3,updated_at=NOW() WHERE id=$1 RETURNING id,name,email,role,is_active`, id, input.Role, input.IsActive).Scan(&input.ID, &input.Name, &input.Email, &input.Role, &input.IsActive)
	if e != nil {
		send(w, 404, map[string]any{"error": map[string]string{"message": "ユーザーが見つかりません"}})
		return
	}
	send(w, 200, input)
}
