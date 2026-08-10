package message

import "time"

type Group struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	CreatedBy     int64      `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	MemberCount   int64      `json:"member_count"`
	LastMessage   *string    `json:"last_message"`
	LastMessageAt *time.Time `json:"last_message_at"`
}

type Message struct {
	ID        int64     `json:"id"`
	GroupID   int64     `json:"group_id"`
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateGroupRequest struct {
	Name        string   `json:"name"`
	MemberNames []string `json:"member_names"`
}

type CreateMessageRequest struct {
	Message string `json:"message"`
}

type GroupListResponse struct {
	Groups  []Group `json:"groups"`
	Limit   int     `json:"limit"`
	Offset  int     `json:"offset"`
	HasMore bool    `json:"has_more"`
}

type MessageListResponse struct {
	Messages []Message `json:"messages"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	HasMore  bool      `json:"has_more"`
}
