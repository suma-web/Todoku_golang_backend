package search

import "time"

type Result struct {
	Type       string     `json:"type"`
	ID         int64      `json:"id"`
	Title      string     `json:"title"`
	Excerpt    string     `json:"excerpt"`
	Category   string     `json:"category,omitempty"`
	Department string     `json:"department,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type Response struct {
	Query   string   `json:"query"`
	Results []Result `json:"results"`
}
