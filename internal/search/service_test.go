package search

import (
	"context"
	"errors"
	"testing"
)

type searchRepositoryStub struct {
	receivedQuery string
}

func (s *searchRepositoryStub) Search(_ context.Context, _ int64, query string) ([]Result, error) {
	s.receivedQuery = query
	return []Result{{Type: "post", ID: 1, Title: "連絡"}}, nil
}

func TestServiceSearchNormalizesQuery(t *testing.T) {
	repository := &searchRepositoryStub{}
	service := NewService(repository)
	response, err := service.Search(context.Background(), 1, "  連絡  ")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if repository.receivedQuery != "連絡" || response.Query != "連絡" || len(response.Results) != 1 {
		t.Fatalf("Search() response = %#v, query = %q", response, repository.receivedQuery)
	}
}

func TestServiceSearchRejectsQueryOver100Runes(t *testing.T) {
	service := NewService(&searchRepositoryStub{})
	query := ""
	for range 101 {
		query += "あ"
	}
	_, err := service.Search(context.Background(), 1, query)
	if !errors.Is(err, ErrQueryTooLong) {
		t.Fatalf("Search() error = %v, want ErrQueryTooLong", err)
	}
}
