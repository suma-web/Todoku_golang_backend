package schoolgroup

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
	"strings"
)

type Group struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}
type Member struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}
type Handler struct{ db *sql.DB }

func NewHandler(db *sql.DB) *Handler { return &Handler{db: db} }
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]any{"error": map[string]string{"message": message}})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,name,type FROM school_groups ORDER BY type,name`)
	if err != nil {
		fail(w, 500, "所属一覧を取得できませんでした")
		return
	}
	defer rows.Close()
	items := []Group{}
	for rows.Next() {
		var item Group
		if rows.Scan(&item.ID, &item.Name, &item.Type) != nil {
			fail(w, 500, "所属一覧を取得できませんでした")
			return
		}
		items = append(items, item)
	}
	respond(w, 200, items)
}
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input Group
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		fail(w, 400, "入力が不正です")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	valid := map[string]bool{"grade": true, "class": true, "club": true, "committee": true, "department": true}
	if input.Name == "" || !valid[input.Type] {
		fail(w, 400, "所属名または種類が不正です")
		return
	}
	if err := h.db.QueryRowContext(r.Context(), `INSERT INTO school_groups(name,type) VALUES($1,$2) RETURNING id`, input.Name, input.Type).Scan(&input.ID); err != nil {
		fail(w, 409, "同じ所属が既に存在します")
		return
	}
	respond(w, 201, input)
}
func idParam(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, key), 10, 64)
}
func (h *Handler) UserGroups(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "userId")
	if err != nil {
		fail(w, 400, "ユーザーIDが不正です")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT g.id,g.name,g.type FROM school_groups g JOIN user_school_groups ug ON ug.group_id=g.id WHERE ug.user_id=$1 ORDER BY g.type,g.name`, id)
	if err != nil {
		fail(w, 500, "所属を取得できませんでした")
		return
	}
	defer rows.Close()
	items := []Group{}
	for rows.Next() {
		var item Group
		_ = rows.Scan(&item.ID, &item.Name, &item.Type)
		items = append(items, item)
	}
	respond(w, 200, items)
}
func (h *Handler) Members(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "groupId")
	if err != nil {
		fail(w, 400, "所属IDが不正です")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT u.id,u.name,u.email,u.role FROM users u JOIN user_school_groups ug ON ug.user_id=u.id WHERE ug.group_id=$1 ORDER BY u.name`, id)
	if err != nil {
		fail(w, 500, "所属メンバーを取得できませんでした")
		return
	}
	defer rows.Close()
	items := []Member{}
	for rows.Next() {
		var item Member
		if rows.Scan(&item.ID, &item.Name, &item.Email, &item.Role) != nil {
			fail(w, 500, "所属メンバーを取得できませんでした")
			return
		}
		items = append(items, item)
	}
	respond(w, 200, items)
}
func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	groupID, err := idParam(r, "groupId")
	if err != nil {
		fail(w, 400, "所属IDが不正です")
		return
	}
	var input struct {
		UserID int64 `json:"user_id"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.UserID < 1 {
		fail(w, 400, "ユーザーIDが不正です")
		return
	}
	_, err = h.db.ExecContext(r.Context(), `INSERT INTO user_school_groups(user_id,group_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, input.UserID, groupID)
	if err != nil {
		fail(w, 404, "ユーザーまたは所属が見つかりません")
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	groupID, e1 := idParam(r, "groupId")
	userID, e2 := idParam(r, "userId")
	if e1 != nil || e2 != nil {
		fail(w, 400, "IDが不正です")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `DELETE FROM user_school_groups WHERE group_id=$1 AND user_id=$2`, groupID, userID)
	if err != nil {
		fail(w, 500, "所属を削除できませんでした")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		fail(w, 404, "所属情報が見つかりません")
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "groupId")
	if err != nil {
		fail(w, 400, "所属IDが不正です")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `DELETE FROM school_groups WHERE id=$1`, id)
	if err != nil {
		fail(w, 500, "所属を削除できませんでした")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		fail(w, 404, "所属が見つかりません")
		return
	}
	w.WriteHeader(204)
}

var _ = errors.Is
