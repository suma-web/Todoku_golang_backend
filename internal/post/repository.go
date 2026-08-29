package post

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, userID int64, doc string, imageURL *string) (Post, error) {
	const query = `
		WITH created AS (
			INSERT INTO posts (user_id, doc, image_url)
			VALUES ($1, $2, $3)
			RETURNING id, user_id, doc, image_url, created_at
		)
		SELECT created.id, created.user_id, users.name, created.doc,
		       created.image_url, created.created_at
		FROM created JOIN users ON users.id = created.user_id`
	var created Post
	err := r.db.QueryRowContext(ctx, query, userID, doc, imageURL).Scan(
		&created.ID, &created.UserID, &created.Name, &created.Doc,
		&created.ImageURL, &created.CreatedAt,
	)
	if err != nil {
		return Post{}, fmt.Errorf("create post: %w", err)
	}
	return created, nil
}

const postSelect = `
	SELECT posts.id, posts.user_id, users.name, posts.doc,
	       posts.image_url, posts.created_at,
	       EXISTS (
	           SELECT 1 FROM bookmarks viewer_bookmarks
	           WHERE viewer_bookmarks.post_id = posts.id
	             AND viewer_bookmarks.user_id = $1
	       )
	FROM posts
	JOIN users ON users.id = posts.user_id`

func scanPost(scanner interface{ Scan(...any) error }) (Post, error) {
	var item Post
	err := scanner.Scan(
		&item.ID, &item.UserID, &item.Name, &item.Doc,
		&item.ImageURL, &item.CreatedAt, &item.BookmarkedByMe,
	)
	return item, err
}

func (r *Repository) List(ctx context.Context, viewerID int64, limit, offset int) ([]Post, error) {
	rows, err := r.db.QueryContext(ctx, postSelect+`
		ORDER BY posts.created_at DESC, posts.id DESC
		LIMIT $2 OFFSET $3`, viewerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()
	return collectPosts(rows)
}

func (r *Repository) ListByUserName(ctx context.Context, viewerID int64, name string, limit, offset int) ([]Post, error) {
	rows, err := r.db.QueryContext(ctx, postSelect+`
		WHERE users.name = $2
		ORDER BY posts.created_at DESC, posts.id DESC
		LIMIT $3 OFFSET $4`, viewerID, name, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list user posts: %w", err)
	}
	defer rows.Close()
	return collectPosts(rows)
}

func collectPosts(rows *sql.Rows) ([]Post, error) {
	items := make([]Post, 0)
	for rows.Next() {
		item, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}
	return items, nil
}

func (r *Repository) FindByID(ctx context.Context, postID, viewerID int64) (Post, error) {
	item, err := scanPost(r.db.QueryRowContext(ctx, postSelect+` WHERE posts.id = $2`, viewerID, postID))
	if err != nil {
		return Post{}, fmt.Errorf("find post by id: %w", err)
	}
	return item, nil
}

func (r *Repository) Bookmark(ctx context.Context, postID, userID int64) (BookmarkResponse, error) {
	const query = `
		INSERT INTO bookmarks (post_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (post_id, user_id) DO NOTHING
		RETURNING id`
	var id int64
	if err := r.db.QueryRowContext(ctx, query, postID, userID).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BookmarkResponse{BookmarkedByMe: true}, ErrAlreadyBookmarked
		}
		return BookmarkResponse{}, fmt.Errorf("bookmark post: %w", err)
	}
	return BookmarkResponse{BookmarkedByMe: true}, nil
}

func (r *Repository) UndoBookmark(ctx context.Context, postID, userID int64) (BookmarkResponse, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM bookmarks WHERE post_id = $1 AND user_id = $2`, postID, userID)
	if err != nil {
		return BookmarkResponse{}, fmt.Errorf("undo bookmark: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return BookmarkResponse{}, fmt.Errorf("count deleted bookmark: %w", err)
	}
	if deleted == 0 {
		return BookmarkResponse{}, ErrNotBookmarked
	}
	return BookmarkResponse{BookmarkedByMe: false}, nil
}

func (r *Repository) ListBookmarkedByUser(ctx context.Context, userID int64, limit, offset int) ([]Post, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT posts.id, posts.user_id, users.name, posts.doc,
		       posts.image_url, posts.created_at, TRUE
		FROM bookmarks my_bookmarks
		JOIN posts ON posts.id = my_bookmarks.post_id
		JOIN users ON users.id = posts.user_id
		WHERE my_bookmarks.user_id = $1
		ORDER BY my_bookmarks.created_at DESC, posts.id DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list bookmarked posts: %w", err)
	}
	defer rows.Close()
	return collectPosts(rows)
}

func (r *Repository) PostExists(ctx context.Context, postID int64) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, postID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check post existence: %w", err)
	}
	return exists, nil
}

var ErrAlreadyBookmarked = errors.New("post already bookmarked")
var ErrNotBookmarked = errors.New("post is not bookmarked")

func (r *Repository) Delete(ctx context.Context, postID, userID int64) (*string, error) {
	const query = `
		DELETE FROM posts
		WHERE id = $1 AND user_id = $2
		RETURNING image_url`
	var imageURL *string
	if err := r.db.QueryRowContext(ctx, query, postID, userID).Scan(&imageURL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("delete post: %w", err)
	}
	return imageURL, nil
}
