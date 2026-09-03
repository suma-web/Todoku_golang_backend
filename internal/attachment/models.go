package attachment

import "time"

const (
	MaxFilesPerParent = 5
	MaxFileBytes      = 10 << 20
	MaxTotalBytes     = 25 << 20
)

type ParentType string

const (
	SchoolPostParent ParentType = "school_post"
	QuestionParent   ParentType = "question"
	AnswerParent     ParentType = "answer"
)

type File struct {
	ID           int64     `json:"id"`
	OriginalName string    `json:"original_name"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
	storageKey   string
}
