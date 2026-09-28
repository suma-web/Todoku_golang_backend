package schoolpost

import "time"

type Post struct {
	PreviousPostID  *int64        `json:"previous_post_id"`
	LatestPostID    *int64        `json:"latest_post_id"`
	Superseded      bool          `json:"superseded"`
	CanViewPrevious bool          `json:"can_view_previous"`
	ChangeSummary   string        `json:"change_summary"`
	Notify          bool          `json:"notify,omitempty"`
	ID              int64         `json:"id"`
	AuthorID        int64         `json:"author_id"`
	AuthorName      string        `json:"author_name"`
	Type            string        `json:"type"`
	Title           string        `json:"title"`
	Content         string        `json:"content"`
	Priority        string        `json:"priority"`
	ExpiresAt       *time.Time    `json:"expires_at"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	GroupIDs        []int64       `json:"group_ids"`
	TargetedByMe    bool          `json:"targeted_by_me"`
	ReadByMe        bool          `json:"read_by_me"`
	ReadUsers       []UserSummary `json:"read_users,omitempty"`
}

type Status struct {
	TargetCount int           `json:"target_count"`
	ReadCount   int           `json:"read_count"`
	UnreadCount int           `json:"unread_count"`
	ReadUsers   []UserSummary `json:"read_users"`
	UnreadUsers []UserSummary `json:"unread_users"`
}

type UserSummary struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
