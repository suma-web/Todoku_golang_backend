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

func (s *Service) Authored(ctx context.Context, userID int64) ([]Post, error) {
	return s.repository.Authored(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, postID, userID int64) error {
	targeted, err := s.repository.IsTarget(ctx, postID, userID)
	if err != nil || !targeted {
		return ErrForbidden
	}
	return s.repository.MarkRead(ctx, postID, userID)
}

func (s *Service) Status(ctx context.Context, postID, userID int64) (Status, error) {
	allowed, err := s.repository.CanViewStatus(ctx, postID, userID)
	if err != nil || !allowed {
		return Status{}, ErrForbidden
	}
	return s.repository.Status(ctx, postID)
}
