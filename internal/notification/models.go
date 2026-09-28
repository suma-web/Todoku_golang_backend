package notification

import "time"

type Notification struct {
	ID              int64      `json:"id"`
	RecipientUserID int64      `json:"recipient_user_id"`
	ActorUserID     *int64     `json:"actor_user_id"`
	PostID          int64      `json:"post_id"`
	Kind            string     `json:"kind"`
	Title           string     `json:"title"`
	ReadAt          *time.Time `json:"read_at"`
	CreatedAt       time.Time  `json:"created_at"`
}
type List struct {
	Items       []Notification `json:"items"`
	UnreadCount int            `json:"unread_count"`
}
