package search

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"twitter_golang_backend/internal/auth"
)

type Result struct {
	Type       string `json:"type"`
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Excerpt    string `json:"excerpt"`
	Category   string `json:"category,omitempty"`
	Department string `json:"department,omitempty"`
}
type Response struct {
	Query   string   `json:"query"`
	Results []Result `json:"results"`
}
type Handler struct{ db *sql.DB }

func NewHandler(db *sql.DB) *Handler { return &Handler{db: db} }
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, 200, Response{Query: query, Results: []Result{}})
		return
	}
	if len([]rune(query)) > 100 {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"message": "検索語は100文字以内にしてください"}})
		return
	}
	uid, _ := auth.UserID(r.Context())
	pattern := "%" + query + "%"
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT type,id,title,excerpt,category,department FROM (
		 SELECT 'post' AS type,p.id,p.title,LEFT(p.content,200) AS excerpt,'' AS category,'' AS department,p.created_at AS sort_at
		 FROM school_posts p WHERE (p.title ILIKE $2 OR p.content ILIKE $2) AND (p.expires_at IS NULL OR p.expires_at>=NOW())
		 AND EXISTS(SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id WHERE pg.post_id=p.id AND ug.user_id=$1)
		 UNION ALL
		 SELECT 'question',q.id,q.title,LEFT(q.content,200),c.name,g.name,q.updated_at FROM questions q JOIN question_categories c ON c.id=q.category_id JOIN school_groups g ON g.id=c.group_id
		 WHERE q.visibility='public' AND(q.title ILIKE $2 OR q.content ILIKE $2 OR EXISTS(SELECT 1 FROM question_answers a WHERE a.question_id=q.id AND a.content ILIKE $2))
		 UNION ALL
		 SELECT 'contact',c.id,c.name,'質問カテゴリの担当窓口',c.name,g.name,c.created_at FROM question_categories c JOIN school_groups g ON g.id=c.group_id WHERE c.name ILIKE $2 OR g.name ILIKE $2
		) results ORDER BY sort_at DESC LIMIT 100`, uid, pattern)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": map[string]string{"message": "検索できませんでした"}})
		return
	}
	defer rows.Close()
	items := []Result{}
	for rows.Next() {
		var item Result
		if rows.Scan(&item.Type, &item.ID, &item.Title, &item.Excerpt, &item.Category, &item.Department) != nil {
			writeJSON(w, 500, map[string]any{"error": map[string]string{"message": "検索できませんでした"}})
			return
		}
		items = append(items, item)
	}
	writeJSON(w, 200, Response{Query: query, Results: items})
}
