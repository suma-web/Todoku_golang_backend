package question

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"twitter_golang_backend/internal/auth"
)

type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
}
type Answer struct {
	ID         int64     `json:"id"`
	QuestionID int64     `json:"question_id"`
	UserID     int64     `json:"user_id"`
	UserName   string    `json:"user_name"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}
type Question struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	UserName       string    `json:"user_name"`
	CategoryID     int64     `json:"category_id"`
	CategoryName   string    `json:"category_name"`
	DepartmentName string    `json:"department_name"`
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	Visibility     string    `json:"visibility"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Answers        []Answer  `json:"answers,omitempty"`
}
type Handler struct{ db *sql.DB }

func NewHandler(db *sql.DB) *Handler { return &Handler{db: db} }
func output(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int, code, message string) {
	output(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func pathID(r *http.Request) (int64, error) { return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64) }

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT c.id,c.name,c.group_id,g.name FROM question_categories c JOIN school_groups g ON g.id=c.group_id ORDER BY c.name`)
	if err != nil {
		failure(w, 500, "INTERNAL_ERROR", "質問カテゴリを取得できませんでした")
		return
	}
	defer rows.Close()
	items := []Category{}
	for rows.Next() {
		var item Category
		if rows.Scan(&item.ID, &item.Name, &item.GroupID, &item.GroupName) != nil {
			failure(w, 500, "INTERNAL_ERROR", "質問カテゴリを取得できませんでした")
			return
		}
		items = append(items, item)
	}
	output(w, 200, items)
}
func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var input Category
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		failure(w, 400, "INVALID_JSON", "入力が不正です")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || input.GroupID < 1 {
		failure(w, 400, "VALIDATION_ERROR", "カテゴリ名と担当部署を指定してください")
		return
	}
	err := h.db.QueryRowContext(r.Context(), `INSERT INTO question_categories(name,group_id) VALUES($1,$2) RETURNING id`, input.Name, input.GroupID).Scan(&input.ID)
	if err != nil {
		failure(w, 409, "CATEGORY_EXISTS", "カテゴリを作成できませんでした")
		return
	}
	output(w, 201, input)
}
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input Question
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		failure(w, 400, "INVALID_JSON", "入力が不正です")
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	if input.Title == "" || input.Content == "" || input.CategoryID < 1 || (input.Visibility != "public" && input.Visibility != "private") {
		failure(w, 400, "VALIDATION_ERROR", "カテゴリ、タイトル、本文、公開範囲を確認してください")
		return
	}
	uid, _ := auth.UserID(r.Context())
	err := h.db.QueryRowContext(r.Context(), `INSERT INTO questions(user_id,category_id,title,content,visibility) VALUES($1,$2,$3,$4,$5) RETURNING id,status,created_at,updated_at`, uid, input.CategoryID, input.Title, input.Content, input.Visibility).Scan(&input.ID, &input.Status, &input.CreatedAt, &input.UpdatedAt)
	if err != nil {
		failure(w, 400, "INVALID_CATEGORY", "質問を作成できませんでした")
		return
	}
	input.UserID = uid
	output(w, 201, input)
}

const questionSelect = `SELECT q.id,q.user_id,u.name,q.category_id,c.name,g.name,q.title,q.content,q.visibility,q.status,q.created_at,q.updated_at FROM questions q JOIN users u ON u.id=q.user_id JOIN question_categories c ON c.id=q.category_id JOIN school_groups g ON g.id=c.group_id`

func scanQuestion(scanner interface{ Scan(...any) error }) (Question, error) {
	var q Question
	err := scanner.Scan(&q.ID, &q.UserID, &q.UserName, &q.CategoryID, &q.CategoryName, &q.DepartmentName, &q.Title, &q.Content, &q.Visibility, &q.Status, &q.CreatedAt, &q.UpdatedAt)
	return q, err
}
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserID(r.Context())
	status := r.URL.Query().Get("status")
	query := questionSelect + ` WHERE (q.user_id=$1 OR q.visibility='public' OR EXISTS(SELECT 1 FROM user_school_groups ug JOIN users viewer ON viewer.id=ug.user_id WHERE ug.user_id=$1 AND ug.group_id=c.group_id AND viewer.role IN('teacher','admin')) OR (SELECT role FROM users WHERE id=$1)='admin') AND ($2='' OR q.status=$2) ORDER BY q.updated_at DESC LIMIT 100`
	rows, err := h.db.QueryContext(r.Context(), query, uid, status)
	if err != nil {
		failure(w, 500, "INTERNAL_ERROR", "質問一覧を取得できませんでした")
		return
	}
	defer rows.Close()
	items := []Question{}
	for rows.Next() {
		q, e := scanQuestion(rows)
		if e != nil {
			failure(w, 500, "INTERNAL_ERROR", "質問一覧を取得できませんでした")
			return
		}
		items = append(items, q)
	}
	output(w, 200, items)
}
func (h *Handler) canAccess(r *http.Request, questionID, uid int64) (bool, error) {
	var allowed bool
	err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM questions q JOIN question_categories c ON c.id=q.category_id JOIN users viewer ON viewer.id=$2 WHERE q.id=$1 AND(q.user_id=$2 OR q.visibility='public' OR viewer.role='admin' OR(viewer.role='teacher' AND EXISTS(SELECT 1 FROM user_school_groups ug WHERE ug.user_id=$2 AND ug.group_id=c.group_id))))`, questionID, uid).Scan(&allowed)
	return allowed, err
}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		failure(w, 400, "INVALID_ID", "質問IDが不正です")
		return
	}
	uid, _ := auth.UserID(r.Context())
	allowed, err := h.canAccess(r, id, uid)
	if err != nil || !allowed {
		failure(w, 404, "QUESTION_NOT_FOUND", "質問が見つかりません")
		return
	}
	q, err := scanQuestion(h.db.QueryRowContext(r.Context(), questionSelect+` WHERE q.id=$1`, id))
	if err != nil {
		failure(w, 404, "QUESTION_NOT_FOUND", "質問が見つかりません")
		return
	}
	q.Answers, err = h.answers(r, id)
	if err != nil {
		failure(w, 500, "INTERNAL_ERROR", "回答を取得できませんでした")
		return
	}
	output(w, 200, q)
}
func (h *Handler) answers(r *http.Request, id int64) ([]Answer, error) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT a.id,a.question_id,a.user_id,u.name,a.content,a.created_at FROM question_answers a JOIN users u ON u.id=a.user_id WHERE a.question_id=$1 ORDER BY a.created_at,a.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Answer{}
	for rows.Next() {
		var a Answer
		if err := rows.Scan(&a.ID, &a.QuestionID, &a.UserID, &a.UserName, &a.Content, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}
func (h *Handler) ListAnswers(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		failure(w, 400, "INVALID_ID", "質問IDが不正です")
		return
	}
	uid, _ := auth.UserID(r.Context())
	allowed, _ := h.canAccess(r, id, uid)
	if !allowed {
		failure(w, 404, "QUESTION_NOT_FOUND", "質問が見つかりません")
		return
	}
	items, err := h.answers(r, id)
	if err != nil {
		failure(w, 500, "INTERNAL_ERROR", "回答を取得できませんでした")
		return
	}
	output(w, 200, items)
}
func (h *Handler) Answer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		failure(w, 400, "INVALID_ID", "質問IDが不正です")
		return
	}
	uid, _ := auth.UserID(r.Context())
	var allowed bool
	err = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM questions q JOIN question_categories c ON c.id=q.category_id JOIN users u ON u.id=$2 WHERE q.id=$1 AND(u.role='admin' OR(u.role='teacher' AND EXISTS(SELECT 1 FROM user_school_groups ug WHERE ug.user_id=$2 AND ug.group_id=c.group_id))))`, id, uid).Scan(&allowed)
	if err != nil || !allowed {
		failure(w, 403, "FORBIDDEN", "この質問へ回答する権限がありません")
		return
	}
	var input Answer
	if json.NewDecoder(r.Body).Decode(&input) != nil || strings.TrimSpace(input.Content) == "" {
		failure(w, 400, "VALIDATION_ERROR", "回答本文を入力してください")
		return
	}
	input.Content = strings.TrimSpace(input.Content)
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, 500, "INTERNAL_ERROR", "回答できませんでした")
		return
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(r.Context(), `INSERT INTO question_answers(question_id,user_id,content) VALUES($1,$2,$3) RETURNING id,created_at`, id, uid, input.Content).Scan(&input.ID, &input.CreatedAt)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE questions SET status='answered',updated_at=NOW() WHERE id=$1 AND status<>'resolved'`, id)
	}
	if err != nil || tx.Commit() != nil {
		failure(w, 500, "INTERNAL_ERROR", "回答できませんでした")
		return
	}
	input.QuestionID = id
	input.UserID = uid
	output(w, 201, input)
}
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		failure(w, 400, "INVALID_ID", "質問IDが不正です")
		return
	}
	uid, _ := auth.UserID(r.Context())
	result, err := h.db.ExecContext(r.Context(), `UPDATE questions SET status='resolved',updated_at=NOW() WHERE id=$1 AND user_id=$2`, id, uid)
	if err != nil {
		failure(w, 500, "INTERNAL_ERROR", "解決済みにできませんでした")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		failure(w, 403, "FORBIDDEN", "質問者本人だけが解決済みにできます")
		return
	}
	w.WriteHeader(204)
}
