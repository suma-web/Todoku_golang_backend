package schooladmin

import (
	"context"
	"database/sql"
	"errors"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("user not found")
)

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.repository.ListUsers(ctx)
}

func (s *Service) UpdateUser(ctx context.Context, id int64, role string, active bool) (User, error) {
	if id < 1 || (role != "student" && role != "teacher" && role != "admin") {
		return User{}, ErrValidation
	}
	item, err := s.repository.UpdateUser(ctx, id, role, active)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return item, err
}
