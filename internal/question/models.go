package question

import "time"

type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
}

type Answer struct {
	ID         int64     `json:"id"`
	QuestionID int64     `json:"question_id"`
	UserID     int64     `json:"user_id"`
	UserName   string    `json:"user_name"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type Question struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	UserName       string    `json:"user_name"`
	CategoryID     int64     `json:"category_id"`
	CategoryName   string    `json:"category_name"`
	DepartmentName string    `json:"department_name"`
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	Visibility     string    `json:"visibility"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Answers        []Answer  `json:"answers,omitempty"`
}
