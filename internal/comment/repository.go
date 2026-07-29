package comment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, postID, userID int64, text string) (Comment, error) {
	const query = `
		WITH created AS (
			INSERT INTO comments (post_id, user_id, comment)
			VALUES ($1, $2, $3)
			RETURNING id, post_id, user_id, comment, created_at
		)
		SELECT created.id, created.post_id, created.user_id, users.name,
		       created.comment, created.created_at
		FROM created
		JOIN users ON users.id = created.user_id`

	var created Comment
	err := r.db.QueryRowContext(ctx, query, postID, userID, text).Scan(
		&created.ID, &created.PostID, &created.UserID, &created.Name,
		&created.Comment, &created.CreatedAt,
	)
	if err != nil {
		return Comment{}, fmt.Errorf("create comment: %w", err)
	}
	return created, nil
}

func (r *Repository) ListByPostID(ctx context.Context, postID int64, limit, offset int) ([]Comment, error) {
	const query = `
		SELECT comments.id, comments.post_id, comments.user_id, users.name,
		       comments.comment, comments.created_at
		FROM comments
		JOIN users ON users.id = comments.user_id
		WHERE comments.post_id = $1
		ORDER BY comments.created_at ASC, comments.id ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, postID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	comments := make([]Comment, 0)
	for rows.Next() {
		var item Comment
		if err := rows.Scan(
			&item.ID, &item.PostID, &item.UserID, &item.Name,
			&item.Comment, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}
	return comments, nil
}

func (r *Repository) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]Comment, error) {
	const query = `
		SELECT comments.id, comments.post_id, comments.user_id, users.name,
		       comments.comment, comments.created_at
		FROM comments
		JOIN users ON users.id = comments.user_id
		WHERE comments.user_id = $1
		ORDER BY comments.created_at DESC, comments.id DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list user comments: %w", err)
	}
	defer rows.Close()

	comments := make([]Comment, 0)
	for rows.Next() {
		var item Comment
		if err := rows.Scan(
			&item.ID, &item.PostID, &item.UserID, &item.Name,
			&item.Comment, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user comment: %w", err)
		}
		comments = append(comments, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user comments: %w", err)
	}
	return comments, nil
}

func (r *Repository) PostExists(ctx context.Context, postID int64) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, postID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check post existence: %w", err)
	}
	return exists, nil
}

func (r *Repository) Delete(ctx context.Context, commentID, userID int64) error {
	const query = `DELETE FROM comments WHERE id = $1 AND user_id = $2 RETURNING id`
	var deletedID int64
	if err := r.db.QueryRowContext(ctx, query, commentID, userID).Scan(&deletedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return fmt.Errorf("delete comment: %w", err)
	}
	return nil
}
