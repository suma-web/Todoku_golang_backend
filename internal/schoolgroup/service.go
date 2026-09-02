package schoolgroup

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrValidation = errors.New("validation error")
	ErrForbidden  = errors.New("forbidden")
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
)

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context) ([]Group, error) { return s.repository.List(ctx) }

func (s *Service) Create(ctx context.Context, input Group) (Group, error) {
	input.Name = strings.TrimSpace(input.Name)
	valid := map[string]bool{"grade": true, "class": true, "club": true, "committee": true, "department": true}
	if input.Name == "" || !valid[input.Type] {
		return Group{}, ErrValidation
	}
	item, err := s.repository.Create(ctx, input)
	return item, classifyDatabaseError(err)
}

func (s *Service) UserGroups(ctx context.Context, viewerID, targetID int64) ([]Group, error) {
	if targetID < 1 {
		return nil, ErrValidation
	}
	allowed, err := s.repository.CanViewUserGroups(ctx, viewerID, targetID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	return s.repository.UserGroups(ctx, targetID)
}

func (s *Service) Members(ctx context.Context, groupID int64) ([]Member, error) {
	if groupID < 1 {
		return nil, ErrValidation
	}
	return s.repository.Members(ctx, groupID)
}

func (s *Service) AddMember(ctx context.Context, groupID, userID int64) error {
	if groupID < 1 || userID < 1 {
		return ErrValidation
	}
	return classifyDatabaseError(s.repository.AddMember(ctx, groupID, userID))
}

func (s *Service) RemoveMember(ctx context.Context, groupID, userID int64) error {
	if groupID < 1 || userID < 1 {
		return ErrValidation
	}
	removed, err := s.repository.RemoveMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !removed {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, groupID int64) error {
	if groupID < 1 {
		return ErrValidation
	}
	deleted, err := s.repository.Delete(ctx, groupID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrConflict
		}
		return err
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

func classifyDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return ErrNotFound
		case "23505", "23514":
			return ErrConflict
		}
	}
	return err
}
