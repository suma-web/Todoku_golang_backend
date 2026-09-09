package schooladmin

import (
	"context"
	"errors"
	"testing"
)

type repositoryStub struct {
	Repository
	updateErr error
}

type repositoryDeleteStub struct {
	repositoryStub
	deletedID int64
	err       error
}

func (s *repositoryStub) UpdateUser(_ context.Context, id int64, role string, active bool) (User, error) {
	return User{ID: id, Role: role, IsActive: active}, s.updateErr
}

func (s *repositoryDeleteStub) DeleteUser(_ context.Context, id int64) error {
	s.deletedID = id
	return s.err
}

func TestUpdateUserRejectsInvalidRole(t *testing.T) {
	service := NewService(&repositoryStub{})
	_, err := service.UpdateUser(context.Background(), 10, 1, "unknown", true)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("UpdateUser() error = %v, want ErrValidation", err)
	}
}

func TestUpdateUserReturnsUpdatedUser(t *testing.T) {
	service := NewService(&repositoryStub{})
	item, err := service.UpdateUser(context.Background(), 10, 1, "teacher", true)
	if err != nil || item.ID != 1 || item.Role != "teacher" {
		t.Fatalf("UpdateUser() item = %#v, error = %v", item, err)
	}
}

func TestUpdateUserRejectsSelfMutation(t *testing.T) {
	service := NewService(&repositoryStub{})
	_, err := service.UpdateUser(context.Background(), 1, 1, "admin", false)
	if !errors.Is(err, ErrSelfMutation) {
		t.Fatalf("UpdateUser() error = %v, want ErrSelfMutation", err)
	}
}

func TestDeleteUserRejectsSelfDeletion(t *testing.T) {
	stub := &repositoryDeleteStub{}
	service := NewService(stub)
	err := service.DeleteUser(context.Background(), 1, 1)
	if !errors.Is(err, ErrSelfMutation) || stub.deletedID != 0 {
		t.Fatalf("DeleteUser() error = %v, deletedID = %d", err, stub.deletedID)
	}
}

func TestDeleteUserDelegatesToRepository(t *testing.T) {
	stub := &repositoryDeleteStub{}
	service := NewService(stub)
	err := service.DeleteUser(context.Background(), 1, 2)
	if err != nil || stub.deletedID != 2 {
		t.Fatalf("DeleteUser() error = %v, deletedID = %d", err, stub.deletedID)
	}
}

func TestUpdateUserReturnsLastAdminError(t *testing.T) {
	service := NewService(&repositoryStub{updateErr: ErrLastActiveAdmin})
	_, err := service.UpdateUser(context.Background(), 1, 2, "teacher", true)
	if !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("UpdateUser() error = %v, want ErrLastActiveAdmin", err)
	}
}

func TestDeleteUserReturnsLastAdminError(t *testing.T) {
	stub := &repositoryDeleteStub{err: ErrLastActiveAdmin}
	service := NewService(stub)
	err := service.DeleteUser(context.Background(), 1, 2)
	if !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("DeleteUser() error = %v, want ErrLastActiveAdmin", err)
	}
}
