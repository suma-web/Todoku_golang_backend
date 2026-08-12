package post

import "time"

type Post struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Name           string    `json:"name"`
	Doc            string    `json:"doc"`
	ImageURL       *string   `json:"image_url"`
	CreatedAt      time.Time `json:"created_at"`
	RetweetCount   int64     `json:"retweet_count"`
	RetweetedByMe  bool      `json:"retweeted_by_me"`
	LikeCount      int64     `json:"like_count"`
	LikedByMe      bool      `json:"liked_by_me"`
	BookmarkedByMe bool      `json:"bookmarked_by_me"`
}

type RetweetResponse struct {
	RetweetCount  int64 `json:"retweet_count"`
	RetweetedByMe bool  `json:"retweeted_by_me"`
}

type LikeResponse struct {
	LikeCount int64 `json:"like_count"`
	LikedByMe bool  `json:"liked_by_me"`
}

type BookmarkResponse struct {
	BookmarkedByMe bool `json:"bookmarked_by_me"`
}

type ListResponse struct {
	Posts   []Post `json:"posts"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	HasMore bool   `json:"has_more"`
}

// UserTweetsResponse leaves room to add profile-related metadata later while
// keeping the user's tweets under a stable key.
type UserTweetsResponse struct {
	Tweets  []Post `json:"tweets"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	HasMore bool   `json:"has_more"`
}
