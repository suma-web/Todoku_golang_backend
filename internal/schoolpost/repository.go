package schoolpost

import (
	"context"
	"database/sql"
)

type Repository interface {
	Create(context.Context, int64, Post) (Post, error)
	Get(context.Context, int64, int64) (Post, error)
	Delete(context.Context, int64, int64) (bool, error)
	Timeline(context.Context, int64) ([]Post, error)
	Authored(context.Context, int64) ([]Post, error)
	IsTarget(context.Context, int64, int64) (bool, error)
	MarkRead(context.Context, int64, int64) error
	CanViewStatus(context.Context, int64, int64) (bool, error)
	Status(context.Context, int64) (Status, error)
}

type SQLRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) Create(ctx context.Context, authorID int64, input Post) (Post, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `INSERT INTO school_posts(author_id,type,title,content,priority,expires_at)VALUES($1,$2,$3,$4,$5,$6)RETURNING id,created_at`,
		authorID, input.Type, input.Title, input.Content, input.Priority, input.ExpiresAt,
	).Scan(&input.ID, &input.CreatedAt)
	if err != nil {
		return Post{}, err
	}
	for _, groupID := range input.GroupIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO school_post_groups VALUES($1,$2)`, input.ID, groupID); err != nil {
			return Post{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Post{}, err
	}
	input.AuthorID = authorID
	return input, nil
}

func (r *SQLRepository) Get(ctx context.Context, postID, userID int64) (Post, error) {
	var item Post
	err := r.db.QueryRowContext(ctx, `SELECT DISTINCT p.id,p.author_id,u.name,p.type,p.title,p.content,p.priority,p.expires_at,p.created_at,
		EXISTS(SELECT 1 FROM school_post_groups target_pg JOIN user_school_groups target_ug ON target_ug.group_id=target_pg.group_id WHERE target_pg.post_id=p.id AND target_ug.user_id=$2),
		EXISTS(SELECT 1 FROM school_post_statuses s WHERE s.post_id=p.id AND s.user_id=$2 AND s.read_at IS NOT NULL)
		FROM school_posts p JOIN users u ON u.id=p.author_id JOIN users viewer ON viewer.id=$2 JOIN school_post_groups pg ON pg.post_id=p.id LEFT JOIN user_school_groups ug ON ug.group_id=pg.group_id
		WHERE p.id=$1 AND (p.author_id=$2 OR viewer.role='admin' OR ((p.expires_at IS NULL OR p.expires_at>=NOW()) AND (ug.user_id=$2 OR viewer.role='teacher')))`, postID, userID).Scan(
		&item.ID, &item.AuthorID, &item.AuthorName, &item.Type, &item.Title,
		&item.Content, &item.Priority, &item.ExpiresAt, &item.CreatedAt, &item.TargetedByMe, &item.ReadByMe,
	)
	return item, err
}

func (r *SQLRepository) Delete(ctx context.Context, postID, userID int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM school_posts WHERE id=$1 AND(author_id=$2 OR(SELECT role FROM users WHERE id=$2)='admin')`, postID, userID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

func (r *SQLRepository) Timeline(ctx context.Context, userID int64) ([]Post, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,author_id,author_name,type,title,content,priority,expires_at,created_at,read_by_me
		FROM (
			SELECT DISTINCT p.id,p.author_id,u.name AS author_name,p.type,p.title,p.content,p.priority,p.expires_at,p.created_at,
			EXISTS(SELECT 1 FROM school_post_statuses s WHERE s.post_id=p.id AND s.user_id=$1 AND s.read_at IS NOT NULL) AS read_by_me
			FROM school_posts p JOIN users u ON u.id=p.author_id JOIN school_post_groups pg ON pg.post_id=p.id
			JOIN user_school_groups ug ON ug.group_id=pg.group_id
			WHERE ug.user_id=$1 AND (p.expires_at IS NULL OR p.expires_at>=NOW())
		) visible_posts
		ORDER BY CASE priority WHEN 'urgent' THEN 1 WHEN 'important' THEN 2 ELSE 3 END,created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Post{}
	for rows.Next() {
		var item Post
		if err := rows.Scan(&item.ID, &item.AuthorID, &item.AuthorName, &item.Type, &item.Title, &item.Content, &item.Priority, &item.ExpiresAt, &item.CreatedAt, &item.ReadByMe); err != nil {
			return nil, err
		}
		item.TargetedByMe = item.AuthorID != userID
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) Authored(ctx context.Context, userID int64) ([]Post, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.id,p.author_id,u.name,p.type,p.title,p.content,p.priority,p.expires_at,p.created_at
		FROM school_posts p JOIN users u ON u.id=p.author_id WHERE p.author_id=$1 ORDER BY p.created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Post{}
	for rows.Next() {
		var item Post
		if err := rows.Scan(&item.ID, &item.AuthorID, &item.AuthorName, &item.Type, &item.Title, &item.Content, &item.Priority, &item.ExpiresAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.ReadUsers, err = r.statusUsers(ctx, item.ID, "read")
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) IsTarget(ctx context.Context, postID, userID int64) (bool, error) {
	var targeted bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id JOIN school_posts p ON p.id=pg.post_id WHERE pg.post_id=$1 AND ug.user_id=$2 AND p.author_id<>$2)`, postID, userID).Scan(&targeted)
	return targeted, err
}

func (r *SQLRepository) MarkRead(ctx context.Context, postID, userID int64) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO school_post_statuses(post_id,user_id,read_at)VALUES($1,$2,NOW()) ON CONFLICT(post_id,user_id)DO UPDATE SET read_at=COALESCE(school_post_statuses.read_at,NOW())`, postID, userID)
	return err
}

func (r *SQLRepository) CanViewStatus(ctx context.Context, postID, userID int64) (bool, error) {
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM school_posts p JOIN users viewer ON viewer.id=$2
		WHERE p.id=$1 AND (p.author_id=$2 OR viewer.role='admin')
	)`, postID, userID).Scan(&allowed)
	return allowed, err
}

func (r *SQLRepository) Status(ctx context.Context, postID int64) (Status, error) {
	var item Status
	err := r.db.QueryRowContext(ctx, `WITH targets AS(SELECT DISTINCT ug.user_id FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id JOIN school_posts p ON p.id=pg.post_id WHERE pg.post_id=$1 AND ug.user_id<>p.author_id) SELECT COUNT(*),COUNT(s.read_at) FROM targets t LEFT JOIN school_post_statuses s ON s.post_id=$1 AND s.user_id=t.user_id`, postID).Scan(
		&item.TargetCount, &item.ReadCount,
	)
	item.UnreadCount = item.TargetCount - item.ReadCount
	if err != nil {
		return item, err
	}
	item.ReadUsers, err = r.statusUsers(ctx, postID, "read")
	if err != nil {
		return item, err
	}
	item.UnreadUsers, err = r.statusUsers(ctx, postID, "unread")
	return item, err
}

func (r *SQLRepository) statusUsers(ctx context.Context, postID int64, state string) ([]UserSummary, error) {
	condition := "s.read_at IS NOT NULL"
	switch state {
	case "unread":
		condition = "s.read_at IS NULL"
	}
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT u.id,u.name
		FROM school_post_groups pg
		JOIN user_school_groups ug ON ug.group_id=pg.group_id
		JOIN school_posts p ON p.id=pg.post_id
		JOIN users u ON u.id=ug.user_id
		LEFT JOIN school_post_statuses s ON s.post_id=pg.post_id AND s.user_id=u.id
		WHERE pg.post_id=$1 AND u.id<>p.author_id AND `+condition+` ORDER BY u.name`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []UserSummary{}
	for rows.Next() {
		var item UserSummary
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
