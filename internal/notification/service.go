package notification

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("notification not found")

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }
func (s *Service) List(ctx context.Context, userID int64) (List, error) {
	return s.repository.List(ctx, userID)
}
func (s *Service) MarkRead(ctx context.Context, id, userID int64) error {
	found, err := s.repository.MarkRead(ctx, id, userID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	return nil
}
