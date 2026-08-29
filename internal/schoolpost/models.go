package schoolpost

import "time"

type Post struct {
	ID         int64      `json:"id"`
	AuthorID   int64      `json:"author_id"`
	AuthorName string     `json:"author_name"`
	Type       string     `json:"type"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	Priority   string     `json:"priority"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	GroupIDs   []int64    `json:"group_ids"`
}

type Status struct {
	TargetCount      int           `json:"target_count"`
	ReadCount        int           `json:"read_count"`
	ConfirmedCount   int           `json:"confirmed_count"`
	UnconfirmedCount int           `json:"unconfirmed_count"`
	ConfirmedUsers   []UserSummary `json:"confirmed_users"`
	ReadOnlyUsers    []UserSummary `json:"read_only_users"`
	UnreadUsers      []UserSummary `json:"unread_users"`
}

type UserSummary struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
