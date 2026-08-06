package notification

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByRecipient(
	ctx context.Context,
	recipientID int64,
	limit, offset int,
) ([]Notification, error) {
	const query = `
		SELECT notifications.id, notifications.kind,
		       notifications.actor_user_id, users.name,
		       notifications.post_id, notifications.comment_id,
		       comments.comment, notifications.created_at
		FROM notifications
		JOIN users ON users.id = notifications.actor_user_id
		LEFT JOIN comments ON comments.id = notifications.comment_id
		WHERE notifications.recipient_user_id = $1
		ORDER BY notifications.created_at DESC, notifications.id DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, recipientID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	notifications := make([]Notification, 0)
	for rows.Next() {
		var item Notification
		if err := rows.Scan(
			&item.ID, &item.Kind, &item.ActorID, &item.ActorName,
			&item.PostID, &item.CommentID, &item.Comment, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		notifications = append(notifications, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notifications: %w", err)
	}
	return notifications, nil
}
