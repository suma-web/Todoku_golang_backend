package schoolpost

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("school post not found")
	ErrForbidden  = errors.New("forbidden")
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, authorID int64, input Post) (Post, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	if input.Title == "" || input.Content == "" || len(input.GroupIDs) == 0 {
		return Post{}, ErrValidation
	}
	return s.repository.Create(ctx, authorID, input)
}

func (s *Service) Get(ctx context.Context, postID, userID int64) (Post, error) {
	item, err := s.repository.Get(ctx, postID, userID)
	if err != nil {
		return Post{}, ErrNotFound
	}
	return item, nil
}

func (s *Service) Delete(ctx context.Context, postID, userID int64) error {
	deleted, err := s.repository.Delete(ctx, postID, userID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Timeline(ctx context.Context, userID int64) ([]Post, error) {
	return s.repository.Timeline(ctx, userID)
}

func (s *Service) Mark(ctx context.Context, postID, userID int64, confirm bool) error {
	targeted, err := s.repository.IsTarget(ctx, postID, userID)
	if err != nil || !targeted {
		return ErrForbidden
	}
	if confirm {
		return s.repository.MarkConfirmed(ctx, postID, userID)
	}
	return s.repository.MarkRead(ctx, postID, userID)
}

func (s *Service) Status(ctx context.Context, postID int64) (Status, error) {
	return s.repository.Status(ctx, postID)
}

func (s *Service) Unconfirmed(ctx context.Context, postID int64) ([]UserSummary, error) {
	return s.repository.Unconfirmed(ctx, postID)
}
