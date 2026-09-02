package schoolpost

import (
	"context"
	"errors"
	"testing"
	"time"
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
	return Status{TargetCount: 3, ReadCount: 1}, nil
}

func TestServiceCreateValidatesAndNormalizesInput(t *testing.T) {
	service := NewService(&schoolPostRepositoryStub{})
	item, err := service.Create(context.Background(), 3, Post{
		Title:    "  校内連絡  ",
		Content:  "  本文  ",
		Type:     "notice",
		Priority: "normal",
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
	_, err := service.Create(context.Background(), 3, Post{Title: "連絡", Content: "本文", Type: "notice", Priority: "normal"})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Create() error = %v, want ErrValidation", err)
	}
}

func TestServiceCreateRejectsPastOrOverTwoYearExpiration(t *testing.T) {
	service := NewService(&schoolPostRepositoryStub{})
	for _, expiresAt := range []time.Time{time.Now().Add(-time.Hour), time.Now().AddDate(2, 0, 1)} {
		_, err := service.Create(context.Background(), 3, Post{Title: "連絡", Content: "本文", Type: "notice", Priority: "normal", GroupIDs: []int64{1}, ExpiresAt: &expiresAt})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Create() expiration %v error = %v, want ErrValidation", expiresAt, err)
		}
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
	if status.TargetCount != 3 || status.ReadCount != 1 {
		t.Fatalf("Status() = %#v", status)
	}
}
