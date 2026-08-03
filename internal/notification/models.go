package notification

import "time"

type Notification struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	ActorID   int64     `json:"actor_user_id"`
	ActorName string    `json:"actor_name"`
	PostID    *int64    `json:"post_id"`
	CommentID *int64    `json:"comment_id"`
	Comment   *string   `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

type ListResponse struct {
	Notifications []Notification `json:"notifications"`
	Limit         int            `json:"limit"`
	Offset        int            `json:"offset"`
	HasMore       bool           `json:"has_more"`
}
