package question

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("question not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	return s.repository.ListCategories(ctx)
}

func (s *Service) CreateCategory(ctx context.Context, input Category) (Category, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || input.GroupID < 1 {
		return Category{}, ErrValidation
	}
	item, err := s.repository.CreateCategory(ctx, input)
	if err != nil {
		return Category{}, ErrConflict
	}
	return item, nil
}

func (s *Service) UpdateCategory(ctx context.Context, input Category) (Category, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.ID < 1 || input.Name == "" || input.GroupID < 1 {
		return Category{}, ErrValidation
	}
	item, err := s.repository.UpdateCategory(ctx, input)
	if err != nil {
		return Category{}, ErrConflict
	}
	return item, nil
}

func (s *Service) Create(ctx context.Context, userID int64, input Question) (Question, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	if input.Title == "" || input.Content == "" || input.CategoryID < 1 ||
		(input.Visibility != "public" && input.Visibility != "private") {
		return Question{}, ErrValidation
	}
	item, err := s.repository.Create(ctx, userID, input)
	if err != nil {
		return Question{}, ErrValidation
	}
	return item, nil
}

func (s *Service) List(ctx context.Context, userID int64, status string) ([]Question, error) {
	return s.repository.List(ctx, userID, status)
}

func (s *Service) Get(ctx context.Context, questionID, userID int64) (Question, error) {
	allowed, err := s.repository.CanAccess(ctx, questionID, userID)
	if err != nil || !allowed {
		return Question{}, ErrNotFound
	}
	item, err := s.repository.Get(ctx, questionID)
	if err != nil {
		return Question{}, ErrNotFound
	}
	item.Answers, err = s.repository.ListAnswers(ctx, questionID)
	return item, err
}

func (s *Service) ListAnswers(ctx context.Context, questionID, userID int64) ([]Answer, error) {
	allowed, err := s.repository.CanAccess(ctx, questionID, userID)
	if err != nil || !allowed {
		return nil, ErrNotFound
	}
	return s.repository.ListAnswers(ctx, questionID)
}

func (s *Service) Answer(ctx context.Context, questionID, userID int64, content string) (Answer, error) {
	allowed, err := s.repository.CanAnswer(ctx, questionID, userID)
	if err != nil || !allowed {
		return Answer{}, ErrForbidden
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return Answer{}, ErrValidation
	}
	return s.repository.CreateAnswer(ctx, questionID, userID, content)
}

func (s *Service) Resolve(ctx context.Context, questionID, userID int64) error {
	updated, err := s.repository.Resolve(ctx, questionID, userID)
	if err != nil {
		return err
	}
	if !updated {
		return ErrForbidden
	}
	return nil
}
