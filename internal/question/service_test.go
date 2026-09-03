package question

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

type questionRepositoryStub struct {
	Repository
	created       Question
	categoryError error
	canAccess     bool
	canAnswer     bool
	getCalled     bool
}

func (s *questionRepositoryStub) UpdateCategory(_ context.Context, input Category) (Category, error) {
	return input, s.categoryError
}

func (s *questionRepositoryStub) Create(_ context.Context, userID int64, input Question) (Question, error) {
	input.UserID = userID
	s.created = input
	return input, nil
}

func (s *questionRepositoryStub) CanAccess(_ context.Context, _, _ int64) (bool, error) {
	return s.canAccess, nil
}

func (s *questionRepositoryStub) CanAnswer(_ context.Context, _, _ int64) (bool, error) {
	return s.canAnswer, nil
}

func (s *questionRepositoryStub) Get(_ context.Context, _ int64) (Question, error) {
	s.getCalled = true
	return Question{}, nil
}

func TestUpdateCategoryReturnsNotFoundSeparately(t *testing.T) {
	service := NewService(&questionRepositoryStub{categoryError: sql.ErrNoRows})
	_, err := service.UpdateCategory(context.Background(), Category{ID: 99, Name: "数学", GroupID: 1})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateCategory() error = %v, want ErrNotFound", err)
	}
}

func TestServiceCreateValidatesAndNormalizesInput(t *testing.T) {
	repository := &questionRepositoryStub{}
	service := NewService(repository)

	item, err := service.Create(context.Background(), 10, Question{
		CategoryID: 1,
		Title:      "  進路相談  ",
		Content:    "  内容  ",
		Visibility: "private",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if item.Title != "進路相談" || item.Content != "内容" || item.UserID != 10 {
		t.Fatalf("Create() item = %#v", item)
	}
}

func TestServiceCreateRejectsInvalidVisibility(t *testing.T) {
	service := NewService(&questionRepositoryStub{})
	_, err := service.Create(context.Background(), 10, Question{
		CategoryID: 1,
		Title:      "質問",
		Content:    "内容",
		Visibility: "unknown",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Create() error = %v, want ErrValidation", err)
	}
}

func TestServiceGetHidesPrivateQuestionFromUnauthorizedViewer(t *testing.T) {
	repository := &questionRepositoryStub{canAccess: false}
	service := NewService(repository)

	_, err := service.Get(context.Background(), 10, 99)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
	if repository.getCalled {
		t.Fatal("Get() must not load a question after access is denied")
	}
}

func TestServiceAnswerRejectsUnauthorizedTeacher(t *testing.T) {
	service := NewService(&questionRepositoryStub{canAnswer: false})

	_, err := service.Answer(context.Background(), 10, 99, "回答")

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Answer() error = %v, want ErrForbidden", err)
	}
}
