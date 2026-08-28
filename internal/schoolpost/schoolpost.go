package schoolpost

import (
	"database/sql"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
	"strings"
	"time"
	"twitter_golang_backend/internal/auth"
)

type Post struct {
	ID         int64      `json:"id"`
	AuthorID   int64      `json:"author_id"`
	AuthorName string     `json:"author_name"`
	Type       string     `json:"type"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	Priority   string     `json:"priority"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	GroupIDs   []int64    `json:"group_ids"`
}
type Handler struct{ db *sql.DB }

func NewHandler(db *sql.DB) *Handler { return &Handler{db} }
func out(w http.ResponseWriter, s int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	_ = json.NewEncoder(w).Encode(v)
}
func bad(w http.ResponseWriter, s int, m string) {
	out(w, s, map[string]any{"error": map[string]string{"message": m}})
}
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var p Post
	if json.NewDecoder(r.Body).Decode(&p) != nil {
		bad(w, 400, "入力が不正です")
		return
	}
	p.Title = strings.TrimSpace(p.Title)
	p.Content = strings.TrimSpace(p.Content)
	if p.Title == "" || p.Content == "" || len(p.GroupIDs) == 0 {
		bad(w, 400, "タイトル、本文、対象所属は必須です")
		return
	}
	id, _ := auth.UserID(r.Context())
	tx, e := h.db.BeginTx(r.Context(), nil)
	if e != nil {
		bad(w, 500, "投稿できませんでした")
		return
	}
	defer tx.Rollback()
	e = tx.QueryRowContext(r.Context(), `INSERT INTO school_posts(author_id,type,title,content,priority,expires_at)VALUES($1,$2,$3,$4,$5,$6)RETURNING id,created_at`, id, p.Type, p.Title, p.Content, p.Priority, p.ExpiresAt).Scan(&p.ID, &p.CreatedAt)
	if e != nil {
		bad(w, 400, "投稿内容が不正です")
		return
	}
	for _, g := range p.GroupIDs {
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO school_post_groups VALUES($1,$2)`, p.ID, g); e != nil {
			bad(w, 400, "対象所属が不正です")
			return
		}
	}
	if tx.Commit() != nil {
		bad(w, 500, "投稿できませんでした")
		return
	}
	p.AuthorID = id
	out(w, 201, p)
}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	pid, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil {
		bad(w, 400, "IDが不正です")
		return
	}
	uid, _ := auth.UserID(r.Context())
	var p Post
	e = h.db.QueryRowContext(r.Context(), `SELECT DISTINCT p.id,p.author_id,u.name,p.type,p.title,p.content,p.priority,p.expires_at,p.created_at FROM school_posts p JOIN users u ON u.id=p.author_id JOIN school_post_groups pg ON pg.post_id=p.id LEFT JOIN user_school_groups ug ON ug.group_id=pg.group_id WHERE p.id=$1 AND(p.author_id=$2 OR ug.user_id=$2 OR(SELECT role FROM users WHERE id=$2)IN('teacher','admin'))`, pid, uid).Scan(&p.ID, &p.AuthorID, &p.AuthorName, &p.Type, &p.Title, &p.Content, &p.Priority, &p.ExpiresAt, &p.CreatedAt)
	if e != nil {
		bad(w, 404, "投稿が見つかりません")
		return
	}
	out(w, 200, p)
}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	pid, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil {
		bad(w, 400, "IDが不正です")
		return
	}
	uid, _ := auth.UserID(r.Context())
	res, e := h.db.ExecContext(r.Context(), `DELETE FROM school_posts WHERE id=$1 AND(author_id=$2 OR(SELECT role FROM users WHERE id=$2)='admin')`, pid, uid)
	if e != nil {
		bad(w, 500, "削除できませんでした")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		bad(w, 404, "投稿が見つからないか権限がありません")
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) Timeline(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserID(r.Context())
	rows, err := h.db.QueryContext(r.Context(), `SELECT DISTINCT p.id,p.author_id,u.name,p.type,p.title,p.content,p.priority,p.expires_at,p.created_at
		FROM school_posts p JOIN users u ON u.id=p.author_id JOIN school_post_groups pg ON pg.post_id=p.id
		JOIN user_school_groups ug ON ug.group_id=pg.group_id
		WHERE ug.user_id=$1 AND (p.expires_at IS NULL OR p.expires_at>=NOW())
		ORDER BY CASE p.priority WHEN 'urgent' THEN 1 WHEN 'important' THEN 2 ELSE 3 END,p.created_at DESC LIMIT 100`, uid)
	if err != nil {
		bad(w, 500, "タイムラインを取得できませんでした")
		return
	}
	defer rows.Close()
	items := []Post{}
	for rows.Next() {
		var p Post
		if rows.Scan(&p.ID, &p.AuthorID, &p.AuthorName, &p.Type, &p.Title, &p.Content, &p.Priority, &p.ExpiresAt, &p.CreatedAt) != nil {
			bad(w, 500, "タイムラインを取得できませんでした")
			return
		}
		items = append(items, p)
	}
	out(w, 200, items)
}

func (h *Handler) Read(w http.ResponseWriter, r *http.Request)    { h.mark(w, r, false) }
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) { h.mark(w, r, true) }
func (h *Handler) mark(w http.ResponseWriter, r *http.Request, confirm bool) {
	pid, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil {
		bad(w, 400, "IDが不正です")
		return
	}
	uid, _ := auth.UserID(r.Context())
	var targeted bool
	e = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id WHERE pg.post_id=$1 AND ug.user_id=$2)`, pid, uid).Scan(&targeted)
	if e != nil || !targeted {
		bad(w, 403, "この投稿の対象者ではありません")
		return
	}
	if confirm {
		_, e = h.db.ExecContext(r.Context(), `INSERT INTO school_post_statuses(post_id,user_id,read_at,confirmed_at)VALUES($1,$2,NOW(),NOW()) ON CONFLICT(post_id,user_id)DO UPDATE SET read_at=COALESCE(school_post_statuses.read_at,NOW()),confirmed_at=NOW()`, pid, uid)
	} else {
		_, e = h.db.ExecContext(r.Context(), `INSERT INTO school_post_statuses(post_id,user_id,read_at)VALUES($1,$2,NOW()) ON CONFLICT(post_id,user_id)DO UPDATE SET read_at=COALESCE(school_post_statuses.read_at,NOW())`, pid, uid)
	}
	if e != nil {
		bad(w, 500, "確認状態を保存できませんでした")
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	pid, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil {
		bad(w, 400, "IDが不正です")
		return
	}
	var target, read, confirmed int
	e = h.db.QueryRowContext(r.Context(), `WITH targets AS(SELECT DISTINCT ug.user_id FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id WHERE pg.post_id=$1) SELECT COUNT(*),COUNT(s.read_at),COUNT(s.confirmed_at) FROM targets t LEFT JOIN school_post_statuses s ON s.post_id=$1 AND s.user_id=t.user_id`, pid).Scan(&target, &read, &confirmed)
	if e != nil {
		bad(w, 500, "確認状況を取得できませんでした")
		return
	}
	out(w, 200, map[string]int{"target_count": target, "read_count": read, "confirmed_count": confirmed, "unconfirmed_count": target - confirmed})
}
func (h *Handler) Unconfirmed(w http.ResponseWriter, r *http.Request) {
	pid, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil {
		bad(w, 400, "IDが不正です")
		return
	}
	rows, e := h.db.QueryContext(r.Context(), `SELECT DISTINCT u.id,u.name FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id JOIN users u ON u.id=ug.user_id LEFT JOIN school_post_statuses s ON s.post_id=pg.post_id AND s.user_id=u.id WHERE pg.post_id=$1 AND s.confirmed_at IS NULL ORDER BY u.name`, pid)
	if e != nil {
		bad(w, 500, "未確認者を取得できませんでした")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name string
		_ = rows.Scan(&id, &name)
		items = append(items, map[string]any{"id": id, "name": name})
	}
	out(w, 200, items)
}
