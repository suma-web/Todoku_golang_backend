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

func (r *Repository) List(ctx context.Context, viewerID int64, limit, offset int) ([]Post, error) {
	const query = `
		SELECT posts.id, posts.user_id, users.name, posts.doc,
		       posts.image_url, posts.created_at, COUNT(retweets.id),
		       EXISTS (
		           SELECT 1 FROM retweets viewer_retweets
		           WHERE viewer_retweets.post_id = posts.id
		             AND viewer_retweets.user_id = $1
		       ),
		       (SELECT COUNT(*) FROM likes WHERE likes.post_id = posts.id),
		       EXISTS (
		           SELECT 1 FROM likes viewer_likes
		           WHERE viewer_likes.post_id = posts.id
		             AND viewer_likes.user_id = $1
		       ),
		       EXISTS (
		           SELECT 1 FROM bookmarks viewer_bookmarks
		           WHERE viewer_bookmarks.post_id = posts.id
		             AND viewer_bookmarks.user_id = $1
		       )
		FROM posts
		JOIN users ON users.id = posts.user_id
		LEFT JOIN retweets ON retweets.post_id = posts.id
		GROUP BY posts.id, users.name
		ORDER BY posts.created_at DESC, posts.id DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, viewerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	posts := make([]Post, 0)
	for rows.Next() {
		var item Post
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Name, &item.Doc,
			&item.ImageURL, &item.CreatedAt, &item.RetweetCount,
			&item.RetweetedByMe, &item.LikeCount, &item.LikedByMe,
			&item.BookmarkedByMe,
		); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}

	return posts, nil
}

func (r *Repository) ListByUserName(ctx context.Context, viewerID int64, name string, limit, offset int) ([]Post, error) {
	const query = `
		SELECT posts.id, posts.user_id, users.name, posts.doc, posts.image_url,
		       posts.created_at, COUNT(retweets.id),
		       EXISTS (
		           SELECT 1 FROM retweets viewer_retweets
		           WHERE viewer_retweets.post_id = posts.id
		             AND viewer_retweets.user_id = $1
		       ),
		       (SELECT COUNT(*) FROM likes WHERE likes.post_id = posts.id),
		       EXISTS (
		           SELECT 1 FROM likes viewer_likes
		           WHERE viewer_likes.post_id = posts.id
		             AND viewer_likes.user_id = $1
		       ),
		       EXISTS (
		           SELECT 1 FROM bookmarks viewer_bookmarks
		           WHERE viewer_bookmarks.post_id = posts.id
		             AND viewer_bookmarks.user_id = $1
		       )
		FROM posts
		JOIN users ON users.id = posts.user_id
		LEFT JOIN retweets ON retweets.post_id = posts.id
		WHERE users.name = $2
		GROUP BY posts.id, users.name
		ORDER BY posts.created_at DESC, posts.id DESC LIMIT $3 OFFSET $4`
	rows, err := r.db.QueryContext(ctx, query, viewerID, name, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list user posts: %w", err)
	}
	defer rows.Close()
	posts := make([]Post, 0)
	for rows.Next() {
		var item Post
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Name, &item.Doc, &item.ImageURL,
			&item.CreatedAt, &item.RetweetCount, &item.RetweetedByMe,
			&item.LikeCount, &item.LikedByMe, &item.BookmarkedByMe,
		); err != nil {
			return nil, fmt.Errorf("scan user post: %w", err)
		}
		posts = append(posts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user posts: %w", err)
	}
	return posts, nil
}

func (r *Repository) FindByID(ctx context.Context, postID, viewerID int64) (Post, error) {
	const query = `
		SELECT posts.id, posts.user_id, users.name, posts.doc,
		       posts.image_url, posts.created_at, COUNT(retweets.id),
		       EXISTS (
		           SELECT 1 FROM retweets viewer_retweets
		           WHERE viewer_retweets.post_id = posts.id
		             AND viewer_retweets.user_id = $2
		       ),
		       (SELECT COUNT(*) FROM likes WHERE likes.post_id = posts.id),
		       EXISTS (
		           SELECT 1 FROM likes viewer_likes
		           WHERE viewer_likes.post_id = posts.id
		             AND viewer_likes.user_id = $2
		       ),
		       EXISTS (
		           SELECT 1 FROM bookmarks viewer_bookmarks
		           WHERE viewer_bookmarks.post_id = posts.id
		             AND viewer_bookmarks.user_id = $2
		       )
		FROM posts
		JOIN users ON users.id = posts.user_id
		LEFT JOIN retweets ON retweets.post_id = posts.id
		WHERE posts.id = $1
		GROUP BY posts.id, users.name`

	var found Post
	err := r.db.QueryRowContext(ctx, query, postID, viewerID).Scan(
		&found.ID, &found.UserID, &found.Name, &found.Doc,
		&found.ImageURL, &found.CreatedAt, &found.RetweetCount,
		&found.RetweetedByMe, &found.LikeCount, &found.LikedByMe,
		&found.BookmarkedByMe,
	)
	if err != nil {
		return Post{}, fmt.Errorf("find post by id: %w", err)
	}

	return found, nil
}

func (r *Repository) Retweet(ctx context.Context, postID, userID int64) (RetweetResponse, error) {
	const query = `
		WITH inserted AS (
			INSERT INTO retweets (post_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT (post_id, user_id) DO NOTHING
			RETURNING id
		)
		SELECT
			(SELECT COUNT(*) FROM retweets WHERE post_id = $1)
			    + (SELECT COUNT(*) FROM inserted),
			EXISTS(SELECT 1 FROM inserted)`

	var response RetweetResponse
	var created bool
	if err := r.db.QueryRowContext(ctx, query, postID, userID).Scan(
		&response.RetweetCount, &created,
	); err != nil {
		return RetweetResponse{}, fmt.Errorf("retweet post: %w", err)
	}
	response.RetweetedByMe = true
	if !created {
		return response, ErrAlreadyRetweeted
	}
	return response, nil
}

func (r *Repository) UndoRetweet(ctx context.Context, postID, userID int64) (RetweetResponse, error) {
	const query = `
		WITH deleted AS (
			DELETE FROM retweets
			WHERE post_id = $1 AND user_id = $2
			RETURNING id
		)
		SELECT
			GREATEST(
				(SELECT COUNT(*) FROM retweets WHERE post_id = $1)
				    - (SELECT COUNT(*) FROM deleted),
				0
			),
			EXISTS(SELECT 1 FROM deleted)`

	var response RetweetResponse
	var deleted bool
	if err := r.db.QueryRowContext(ctx, query, postID, userID).Scan(
		&response.RetweetCount, &deleted,
	); err != nil {
		return RetweetResponse{}, fmt.Errorf("undo retweet: %w", err)
	}
	if !deleted {
		return response, ErrNotRetweeted
	}
	return response, nil
}

func (r *Repository) ListRetweetedByUser(ctx context.Context, userID int64, limit, offset int) ([]Post, error) {
	const query = `
		SELECT posts.id, posts.user_id, users.name, posts.doc,
		       posts.image_url, posts.created_at, COUNT(all_retweets.id), TRUE,
		       (SELECT COUNT(*) FROM likes WHERE likes.post_id = posts.id),
		       EXISTS (
		           SELECT 1 FROM likes viewer_likes
		           WHERE viewer_likes.post_id = posts.id
		             AND viewer_likes.user_id = $1
		       ),
		       EXISTS (
		           SELECT 1 FROM bookmarks viewer_bookmarks
		           WHERE viewer_bookmarks.post_id = posts.id
		             AND viewer_bookmarks.user_id = $1
		       )
		FROM retweets my_retweets
		JOIN posts ON posts.id = my_retweets.post_id
		JOIN users ON users.id = posts.user_id
		LEFT JOIN retweets all_retweets ON all_retweets.post_id = posts.id
		WHERE my_retweets.user_id = $1
		GROUP BY posts.id, users.name, my_retweets.created_at
		ORDER BY my_retweets.created_at DESC, posts.id DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list retweeted posts: %w", err)
	}
	defer rows.Close()

	posts := make([]Post, 0)
	for rows.Next() {
		var item Post
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Name, &item.Doc,
			&item.ImageURL, &item.CreatedAt, &item.RetweetCount,
			&item.RetweetedByMe, &item.LikeCount, &item.LikedByMe,
			&item.BookmarkedByMe,
		); err != nil {
			return nil, fmt.Errorf("scan retweeted post: %w", err)
		}
		posts = append(posts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retweeted posts: %w", err)
	}
	return posts, nil
}

func (r *Repository) Like(ctx context.Context, postID, userID int64) (LikeResponse, error) {
	const query = `
		WITH inserted AS (
			INSERT INTO likes (post_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT (post_id, user_id) DO NOTHING
			RETURNING id
		)
		SELECT
			(SELECT COUNT(*) FROM likes WHERE post_id = $1)
			    + (SELECT COUNT(*) FROM inserted),
			EXISTS(SELECT 1 FROM inserted)`

	var response LikeResponse
	var created bool
	if err := r.db.QueryRowContext(ctx, query, postID, userID).Scan(
		&response.LikeCount, &created,
	); err != nil {
		return LikeResponse{}, fmt.Errorf("like post: %w", err)
	}
	response.LikedByMe = true
	if !created {
		return response, ErrAlreadyLiked
	}
	return response, nil
}

func (r *Repository) UndoLike(ctx context.Context, postID, userID int64) (LikeResponse, error) {
	const query = `
		WITH deleted AS (
			DELETE FROM likes
			WHERE post_id = $1 AND user_id = $2
			RETURNING id
		)
		SELECT
			GREATEST(
				(SELECT COUNT(*) FROM likes WHERE post_id = $1)
				    - (SELECT COUNT(*) FROM deleted),
				0
			),
			EXISTS(SELECT 1 FROM deleted)`

	var response LikeResponse
	var deleted bool
	if err := r.db.QueryRowContext(ctx, query, postID, userID).Scan(
		&response.LikeCount, &deleted,
	); err != nil {
		return LikeResponse{}, fmt.Errorf("undo like: %w", err)
	}
	if !deleted {
		return response, ErrNotLiked
	}
	return response, nil
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
	const query = `
		SELECT posts.id, posts.user_id, users.name, posts.doc,
		       posts.image_url, posts.created_at, COUNT(all_retweets.id),
		       EXISTS (
		           SELECT 1 FROM retweets viewer_retweets
		           WHERE viewer_retweets.post_id = posts.id
		             AND viewer_retweets.user_id = $1
		       ),
		       (SELECT COUNT(*) FROM likes WHERE likes.post_id = posts.id),
		       EXISTS (
		           SELECT 1 FROM likes viewer_likes
		           WHERE viewer_likes.post_id = posts.id
		             AND viewer_likes.user_id = $1
		       ), TRUE
		FROM bookmarks my_bookmarks
		JOIN posts ON posts.id = my_bookmarks.post_id
		JOIN users ON users.id = posts.user_id
		LEFT JOIN retweets all_retweets ON all_retweets.post_id = posts.id
		WHERE my_bookmarks.user_id = $1
		GROUP BY posts.id, users.name, my_bookmarks.created_at
		ORDER BY my_bookmarks.created_at DESC, posts.id DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list bookmarked posts: %w", err)
	}
	defer rows.Close()
	posts := make([]Post, 0)
	for rows.Next() {
		var item Post
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Name, &item.Doc,
			&item.ImageURL, &item.CreatedAt, &item.RetweetCount,
			&item.RetweetedByMe, &item.LikeCount, &item.LikedByMe,
			&item.BookmarkedByMe,
		); err != nil {
			return nil, fmt.Errorf("scan bookmarked post: %w", err)
		}
		posts = append(posts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookmarked posts: %w", err)
	}
	return posts, nil
}

func (r *Repository) PostExists(ctx context.Context, postID int64) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, postID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check post existence: %w", err)
	}
	return exists, nil
}

var ErrAlreadyRetweeted = errors.New("post already retweeted")
var ErrNotRetweeted = errors.New("post is not retweeted")
var ErrAlreadyLiked = errors.New("post already liked")
var ErrNotLiked = errors.New("post is not liked")
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
