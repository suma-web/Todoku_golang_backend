package search

import (
	"context"
	"errors"
	"strings"
)

var ErrQueryTooLong = errors.New("search query is too long")

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Search(ctx context.Context, userID int64, query string) (Response, error) {
	query = strings.TrimSpace(query)
	response := Response{Query: query, Results: []Result{}}
	if query == "" {
		return response, nil
	}
	if len([]rune(query)) > 100 {
		return Response{}, ErrQueryTooLong
	}
	items, err := s.repository.Search(ctx, userID, query)
	if err != nil {
		return Response{}, err
	}
	response.Results = items
	return response, nil
}
