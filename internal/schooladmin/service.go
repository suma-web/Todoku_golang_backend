package schooladmin

import (
	"context"
	"database/sql"
	"errors"
)

var (
	ErrValidation      = errors.New("validation error")
	ErrNotFound        = errors.New("user not found")
	ErrSelfMutation    = errors.New("cannot change or delete yourself")
	ErrLastActiveAdmin = errors.New("at least one active admin is required")
)

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.repository.ListUsers(ctx)
}

func (s *Service) UpdateUser(ctx context.Context, actorID, targetID int64, role string, active bool) (User, error) {
	if actorID < 1 || targetID < 1 || (role != "student" && role != "teacher" && role != "admin") {
		return User{}, ErrValidation
	}
	if actorID == targetID {
		return User{}, ErrSelfMutation
	}
	item, err := s.repository.UpdateUser(ctx, targetID, role, active)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return item, err
}

func (s *Service) DeleteUser(ctx context.Context, actorID, targetID int64) error {
	if actorID < 1 || targetID < 1 {
		return ErrValidation
	}
	if actorID == targetID {
		return ErrSelfMutation
	}
	if err := s.repository.DeleteUser(ctx, targetID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else {
		return err
	}
}
