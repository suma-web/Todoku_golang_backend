package schoolpost

import (
	"context"
	"database/sql"
)

type Repository interface {
	Create(context.Context, int64, Post) (Post, error)
	Update(context.Context, int64, int64, Post) (Post, error)
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

	err = tx.QueryRowContext(ctx, `INSERT INTO school_posts(author_id,type,title,content,priority,expires_at)VALUES($1,$2,$3,$4,$5,$6)RETURNING id,created_at,updated_at`,
		authorID, input.Type, input.Title, input.Content, input.Priority, input.ExpiresAt,
	).Scan(&input.ID, &input.CreatedAt, &input.UpdatedAt)
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

func (r *SQLRepository) Update(ctx context.Context, postID, userID int64, input Post) (Post, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback()

	// Serialize edits/reposts on their source so a second request cannot branch history.
	if err = tx.QueryRowContext(ctx, `SELECT author_id FROM school_posts WHERE id=$1 AND (author_id=$2 OR (SELECT role FROM users WHERE id=$2)='admin') FOR UPDATE`, postID, userID).Scan(&input.AuthorID); err != nil {
		return Post{}, err
	}
	var superseded bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM school_posts WHERE previous_post_id=$1)`, postID).Scan(&superseded); err != nil {
		return Post{}, err
	}
	if superseded {
		return Post{}, ErrConflict
	}
	if input.Notify {
		input.PreviousPostID = &postID
		err = tx.QueryRowContext(ctx, `INSERT INTO school_posts(author_id,type,title,content,priority,expires_at,previous_post_id,change_summary)
        VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,created_at,updated_at`, input.AuthorID, input.Type, input.Title, input.Content, input.Priority, input.ExpiresAt, postID, input.ChangeSummary).Scan(&input.ID, &input.CreatedAt, &input.UpdatedAt)
	} else {
		input.ID = postID
		err = tx.QueryRowContext(ctx, `UPDATE school_posts SET type=$3,title=$4,content=$5,priority=$6,expires_at=$7,updated_at=NOW()
        WHERE id=$1 AND author_id=$2 RETURNING created_at,updated_at,previous_post_id,change_summary`, postID, input.AuthorID, input.Type, input.Title, input.Content, input.Priority, input.ExpiresAt).Scan(&input.CreatedAt, &input.UpdatedAt, &input.PreviousPostID, &input.ChangeSummary)
	}
	if err != nil {
		return Post{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM school_post_groups WHERE post_id=$1`, input.ID); err != nil {
		return Post{}, err
	}
	for _, groupID := range input.GroupIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO school_post_groups(post_id,group_id) VALUES($1,$2)`, input.ID, groupID); err != nil {
			return Post{}, err
		}
	}
	if input.Notify {
		// Use the updated audience, deduplicating users in multiple groups.
		_, err = tx.ExecContext(ctx, `INSERT INTO notifications(recipient_user_id,actor_user_id,post_id,kind)
            SELECT DISTINCT u.id,$2::bigint,p.id,'important_update'
            FROM school_posts p JOIN school_post_groups pg ON pg.post_id=p.id
            JOIN user_school_groups ug ON ug.group_id=pg.group_id JOIN users u ON u.id=ug.user_id
            WHERE p.id=$1 AND u.id<>p.author_id AND u.id<>$2 AND u.is_active AND u.deleted_at IS NULL`, input.ID, userID)
		if err != nil {
			return Post{}, err
		}
	}
	if err = tx.QueryRowContext(ctx, `SELECT name FROM users WHERE id=$1`, input.AuthorID).Scan(&input.AuthorName); err != nil {
		return Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return Post{}, err
	}
	return input, nil
}

func (r *SQLRepository) Get(ctx context.Context, postID, userID int64) (Post, error) {
	var item Post
	err := r.db.QueryRowContext(ctx, `SELECT p.id,p.author_id,u.name,p.type,p.title,p.content,p.priority,p.expires_at,p.created_at,p.updated_at,
		EXISTS(SELECT 1 FROM school_post_groups target_pg JOIN user_school_groups target_ug ON target_ug.group_id=target_pg.group_id WHERE target_pg.post_id=p.id AND target_ug.user_id=$2),
		EXISTS(SELECT 1 FROM school_post_statuses s WHERE s.post_id=p.id AND s.user_id=$2 AND s.read_at IS NOT NULL),
 p.previous_post_id,p.change_summary,EXISTS(SELECT 1 FROM school_posts successor WHERE successor.previous_post_id=p.id)
		FROM school_posts p JOIN users u ON u.id=p.author_id JOIN users viewer ON viewer.id=$2
		WHERE p.id=$1 AND (p.author_id=$2 OR viewer.role='admin' OR ((p.expires_at IS NULL OR p.expires_at>=NOW()) AND EXISTS(
			SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id WHERE pg.post_id=p.id AND ug.user_id=$2
		)))`, postID, userID).Scan(
		&item.ID, &item.AuthorID, &item.AuthorName, &item.Type, &item.Title,
		&item.Content, &item.Priority, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt, &item.TargetedByMe, &item.ReadByMe,
		&item.PreviousPostID, &item.ChangeSummary, &item.Superseded,
	)
	if err == nil {
		item.GroupIDs, err = r.groupIDs(ctx, item.ID)
	}
	if err == nil {
		err = r.historyLinks(ctx, &item, userID)
	}
	return item, err
}

func (r *SQLRepository) historyLinks(ctx context.Context, item *Post, userID int64) error {
	const access = `(p.author_id=$2 OR (SELECT role FROM users WHERE id=$2)='admin' OR ((p.expires_at IS NULL OR p.expires_at>=NOW()) AND EXISTS(SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id WHERE pg.post_id=p.id AND ug.user_id=$2)))`
	if item.PreviousPostID != nil {
		if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM school_posts p WHERE p.id=$1 AND `+access+`)`, *item.PreviousPostID, userID).Scan(&item.CanViewPrevious); err != nil {
			return err
		}
	}
	if item.Superseded {
		var id int64
		err := r.db.QueryRowContext(ctx, `WITH RECURSIVE chain AS (
          SELECT id FROM school_posts WHERE id=$1 UNION ALL SELECT p.id FROM school_posts p JOIN chain c ON p.previous_post_id=c.id
        ) SELECT p.id FROM school_posts p JOIN chain c ON c.id=p.id WHERE NOT EXISTS(SELECT 1 FROM school_posts n WHERE n.previous_post_id=p.id) AND `+access, item.ID, userID).Scan(&id)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		item.LatestPostID = &id
	}
	return nil
}

func (r *SQLRepository) Delete(ctx context.Context, postID, userID int64) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var previous *int64
	err = tx.QueryRowContext(ctx, `SELECT previous_post_id FROM school_posts WHERE id=$1 AND(author_id=$2 OR(SELECT role FROM users WHERE id=$2)='admin') FOR UPDATE`, postID, userID).Scan(&previous)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var hasNext bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM school_posts WHERE previous_post_id=$1)`, postID).Scan(&hasNext); err != nil {
		return false, err
	}
	if previous != nil || hasNext {
		return false, ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM school_posts WHERE id=$1`, postID); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (r *SQLRepository) Timeline(ctx context.Context, userID int64) ([]Post, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.id,p.author_id,u.name,p.type,p.title,p.content,p.priority,p.expires_at,p.created_at,p.updated_at,
		EXISTS(SELECT 1 FROM school_post_statuses s WHERE s.post_id=p.id AND s.user_id=$1 AND s.read_at IS NOT NULL),
		EXISTS(SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id WHERE pg.post_id=p.id AND ug.user_id=$1 AND p.author_id<>$1)
		FROM school_posts p JOIN users u ON u.id=p.author_id JOIN users viewer ON viewer.id=$1
		WHERE NOT EXISTS(SELECT 1 FROM school_posts n WHERE n.previous_post_id=p.id) AND (viewer.role='admin' OR ((p.expires_at IS NULL OR p.expires_at>=NOW()) AND EXISTS(
			SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id WHERE pg.post_id=p.id AND ug.user_id=$1
		))) ORDER BY CASE p.priority WHEN 'urgent' THEN 1 WHEN 'important' THEN 2 ELSE 3 END,p.created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Post{}
	for rows.Next() {
		var item Post
		if err := rows.Scan(&item.ID, &item.AuthorID, &item.AuthorName, &item.Type, &item.Title, &item.Content, &item.Priority, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt, &item.ReadByMe, &item.TargetedByMe); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range items {
		items[i].GroupIDs, err = r.groupIDs(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (r *SQLRepository) Authored(ctx context.Context, userID int64) ([]Post, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.id,p.author_id,u.name,p.type,p.title,p.content,p.priority,p.expires_at,p.created_at,p.updated_at
		FROM school_posts p JOIN users u ON u.id=p.author_id WHERE p.author_id=$1 AND NOT EXISTS(SELECT 1 FROM school_posts n WHERE n.previous_post_id=p.id) ORDER BY p.created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Post{}
	for rows.Next() {
		var item Post
		if err := rows.Scan(&item.ID, &item.AuthorID, &item.AuthorName, &item.Type, &item.Title, &item.Content, &item.Priority, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range items {
		items[i].GroupIDs, err = r.groupIDs(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
	}
	for i := range items {
		items[i].ReadUsers, err = r.statusUsers(ctx, items[i].ID, "read")
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (r *SQLRepository) IsTarget(ctx context.Context, postID, userID int64) (bool, error) {
	var targeted bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id JOIN school_posts p ON p.id=pg.post_id WHERE pg.post_id=$1 AND ug.user_id=$2 AND p.author_id<>$2 AND (p.expires_at IS NULL OR p.expires_at>=NOW()))`, postID, userID).Scan(&targeted)
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
	err := r.db.QueryRowContext(ctx, `WITH targets AS(SELECT DISTINCT ug.user_id FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id JOIN school_posts p ON p.id=pg.post_id JOIN users u ON u.id=ug.user_id WHERE pg.post_id=$1 AND ug.user_id<>p.author_id AND u.is_active) SELECT COUNT(*),COUNT(s.read_at) FROM targets t LEFT JOIN school_post_statuses s ON s.post_id=$1 AND s.user_id=t.user_id`, postID).Scan(
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
		WHERE pg.post_id=$1 AND u.id<>p.author_id AND u.is_active AND `+condition+` ORDER BY u.name`, postID)
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

func (r *SQLRepository) groupIDs(ctx context.Context, postID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT group_id FROM school_post_groups WHERE post_id=$1 ORDER BY group_id`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
