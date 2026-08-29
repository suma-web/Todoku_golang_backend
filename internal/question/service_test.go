package question

import (
	"context"
	"errors"
	"testing"
)

type questionRepositoryStub struct {
	Repository
	created Question
}

func (s *questionRepositoryStub) Create(_ context.Context, userID int64, input Question) (Question, error) {
	input.UserID = userID
	s.created = input
	return input, nil
}

func TestServiceCreateValidatesAndNormalizesInput(t *testing.T) {
	repository := &questionRepositoryStub{}
	service := NewService(repository)

	item, err := service.Create(context.Background(), 10, Question{
		CategoryID: 1,
		Title:      "  進路相談  ",
		Content:    "  内容  ",
		Visibility: "private",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if item.Title != "進路相談" || item.Content != "内容" || item.UserID != 10 {
		t.Fatalf("Create() item = %#v", item)
	}
}

func TestServiceCreateRejectsInvalidVisibility(t *testing.T) {
	service := NewService(&questionRepositoryStub{})
	_, err := service.Create(context.Background(), 10, Question{
		CategoryID: 1,
		Title:      "質問",
		Content:    "内容",
		Visibility: "unknown",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Create() error = %v, want ErrValidation", err)
	}
}
