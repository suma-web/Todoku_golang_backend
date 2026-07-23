package comment

import "time"

type Comment struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateRequest struct {
	Comment string `json:"comment"`
}

type ListResponse struct {
	Comments []Comment `json:"comments"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	HasMore  bool      `json:"has_more"`
}
