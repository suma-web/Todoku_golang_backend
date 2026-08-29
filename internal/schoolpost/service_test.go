package schoolpost

import (
	"context"
	"errors"
	"testing"
)

type schoolPostRepositoryStub struct {
	Repository
	canViewStatus bool
}

func (s *schoolPostRepositoryStub) Create(_ context.Context, authorID int64, input Post) (Post, error) {
	input.AuthorID = authorID
	return input, nil
}

func (s *schoolPostRepositoryStub) CanViewStatus(_ context.Context, _, _ int64) (bool, error) {
	return s.canViewStatus, nil
}

func (s *schoolPostRepositoryStub) Status(_ context.Context, _ int64) (Status, error) {
	return Status{TargetCount: 3, ConfirmedCount: 1}, nil
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

func TestServiceStatusRequiresAuthorOrAdmin(t *testing.T) {
	service := NewService(&schoolPostRepositoryStub{canViewStatus: false})
	if _, err := service.Status(context.Background(), 10, 20); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Status() error = %v, want ErrForbidden", err)
	}
}

func TestServiceStatusReturnsDetailsForAllowedViewer(t *testing.T) {
	service := NewService(&schoolPostRepositoryStub{canViewStatus: true})
	status, err := service.Status(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.TargetCount != 3 || status.ConfirmedCount != 1 {
		t.Fatalf("Status() = %#v", status)
	}
}
