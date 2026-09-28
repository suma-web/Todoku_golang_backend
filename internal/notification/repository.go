package notification

import (
	"context"
	"database/sql"
)

type Repository interface {
	List(context.Context, int64) (List, error)
	MarkRead(context.Context, int64, int64) (bool, error)
}
type SQLRepository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return &SQLRepository{db: db} }

// Apply current post visibility to both the list and count so removed members
// cannot see titles, and expired posts do not leave inaccessible unread badges.
const visible = ` FROM notifications n JOIN school_posts p ON p.id=n.post_id
 JOIN users viewer ON viewer.id=n.recipient_user_id
 WHERE n.recipient_user_id=$1 AND viewer.is_active AND viewer.deleted_at IS NULL
 AND (p.author_id=$1 OR viewer.role='admin' OR ((p.expires_at IS NULL OR p.expires_at>=NOW()) AND EXISTS(
 SELECT 1 FROM school_post_groups pg JOIN user_school_groups ug ON ug.group_id=pg.group_id
 WHERE pg.post_id=p.id AND ug.user_id=$1)))`

func (r *SQLRepository) List(ctx context.Context, userID int64) (List, error) {
	result := List{Items: []Notification{}}
	// One snapshot keeps the total consistent with the returned page.
	rows, err := r.db.QueryContext(ctx, `SELECT n.id,n.recipient_user_id,n.actor_user_id,n.post_id,n.kind,p.title,n.read_at,n.created_at,
 COUNT(*) FILTER (WHERE n.read_at IS NULL) OVER ()`+visible+` ORDER BY n.created_at DESC,n.id DESC LIMIT 100`, userID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Notification
		if err := rows.Scan(&item.ID, &item.RecipientUserID, &item.ActorUserID, &item.PostID, &item.Kind, &item.Title, &item.ReadAt, &item.CreatedAt, &result.UnreadCount); err != nil {
			return result, err
		}
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}
func (r *SQLRepository) MarkRead(ctx context.Context, id, userID int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,NOW()) WHERE id=$2 AND id IN (SELECT n.id`+visible+`)`, userID, id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
