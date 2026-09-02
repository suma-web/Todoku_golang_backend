package schoolgroup

import (
	"context"
	"errors"
	"testing"
)

type repositoryStub struct {
	Repository
	canView bool
}

func (s *repositoryStub) CanViewUserGroups(context.Context, int64, int64) (bool, error) {
	return s.canView, nil
}

func (s *repositoryStub) UserGroups(context.Context, int64) ([]Group, error) {
	return []Group{{ID: 1, Name: "2年A組", Type: "class"}}, nil
}

func TestUserGroupsRejectsUnauthorizedViewer(t *testing.T) {
	service := NewService(&repositoryStub{canView: false})
	_, err := service.UserGroups(context.Background(), 10, 20)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("UserGroups() error = %v, want ErrForbidden", err)
	}
}

func TestUserGroupsReturnsGroupsForAuthorizedViewer(t *testing.T) {
	service := NewService(&repositoryStub{canView: true})
	items, err := service.UserGroups(context.Background(), 10, 20)
	if err != nil || len(items) != 1 || items[0].Name != "2年A組" {
		t.Fatalf("UserGroups() items = %#v, error = %v", items, err)
	}
}
