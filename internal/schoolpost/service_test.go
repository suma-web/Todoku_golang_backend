package schoolpost

import (
	"context"
	"errors"
	"testing"
)

type schoolPostRepositoryStub struct {
	Repository
}

func (s *schoolPostRepositoryStub) Create(_ context.Context, authorID int64, input Post) (Post, error) {
	input.AuthorID = authorID
	return input, nil
}

func TestServiceCreateValidatesAndNormalizesInput(t *testing.T) {
	service := NewService(&schoolPostRepositoryStub{})
	item, err := service.Create(context.Background(), 3, Post{
		Title:    "  校内連絡  ",
		Content:  "  本文  ",
		GroupIDs: []int64{1},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if item.Title != "校内連絡" || item.Content != "本文" || item.AuthorID != 3 {
		t.Fatalf("Create() item = %#v", item)
	}
}

func TestServiceCreateRequiresTargetGroup(t *testing.T) {
	service := NewService(&schoolPostRepositoryStub{})
	_, err := service.Create(context.Background(), 3, Post{Title: "連絡", Content: "本文"})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Create() error = %v, want ErrValidation", err)
	}
}
