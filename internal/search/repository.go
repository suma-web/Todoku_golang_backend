package search

import (
	"context"
	"database/sql"
)

type Repository interface {
	Search(context.Context, int64, string) ([]Result, error)
}

type SQLRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) Search(ctx context.Context, userID int64, query string) ([]Result, error) {
	pattern := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, `
		SELECT type,id,title,excerpt,category,department FROM (
		 SELECT 'post' AS type,p.id,p.title,LEFT(p.content,200) AS excerpt,'' AS category,'' AS department,p.created_at AS sort_at
		 FROM school_posts p WHERE (p.title ILIKE $2 OR p.content ILIKE $2) AND (p.expires_at IS NULL OR p.expires_at>=NOW())
		 AND EXISTS(SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id WHERE pg.post_id=p.id AND ug.user_id=$1)
		 UNION ALL
		 SELECT 'question',q.id,q.title,LEFT(q.content,200),c.name,g.name,q.updated_at FROM questions q JOIN question_categories c ON c.id=q.category_id JOIN school_groups g ON g.id=c.group_id
		 WHERE q.visibility='public' AND(q.title ILIKE $2 OR q.content ILIKE $2 OR EXISTS(SELECT 1 FROM question_answers a WHERE a.question_id=q.id AND a.content ILIKE $2))
		 UNION ALL
		 SELECT 'contact',c.id,c.name,'質問カテゴリの担当窓口',c.name,g.name,c.created_at FROM question_categories c JOIN school_groups g ON g.id=c.group_id WHERE c.name ILIKE $2 OR g.name ILIKE $2
		) results ORDER BY sort_at DESC LIMIT 100`, userID, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Result{}
	for rows.Next() {
		var item Result
		if err := rows.Scan(&item.Type, &item.ID, &item.Title, &item.Excerpt, &item.Category, &item.Department); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
