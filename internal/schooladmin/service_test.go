package schooladmin

import (
	"context"
	"errors"
	"testing"
)

type repositoryStub struct{ Repository }

func (s *repositoryStub) UpdateUser(_ context.Context, id int64, role string, active bool) (User, error) {
	return User{ID: id, Role: role, IsActive: active}, nil
}

func TestUpdateUserRejectsInvalidRole(t *testing.T) {
	service := NewService(&repositoryStub{})
	_, err := service.UpdateUser(context.Background(), 1, "unknown", true)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("UpdateUser() error = %v, want ErrValidation", err)
	}
}

func TestUpdateUserReturnsUpdatedUser(t *testing.T) {
	service := NewService(&repositoryStub{})
	item, err := service.UpdateUser(context.Background(), 1, "teacher", true)
	if err != nil || item.ID != 1 || item.Role != "teacher" {
		t.Fatalf("UpdateUser() item = %#v, error = %v", item, err)
	}
}
