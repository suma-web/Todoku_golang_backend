package schooladmin

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRepositoryPreventsRemovingLastActiveAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM users WHERE role='admin' AND is_active=TRUE AND deleted_at IS NULL ORDER BY id FOR UPDATE`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT role,is_active FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "is_active"}).AddRow("admin", true))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM users WHERE role='admin' AND is_active=TRUE AND deleted_at IS NULL`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err = repository.UpdateUser(context.Background(), 1, "teacher", true)
	if !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("UpdateUser() error = %v, want ErrLastActiveAdmin", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositorySoftDeletesAndRemovesMemberships(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM users WHERE role='admin' AND is_active=TRUE AND deleted_at IS NULL ORDER BY id FOR UPDATE`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT role,is_active FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "is_active"}).AddRow("student", true))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM user_school_groups WHERE user_id=$1`)).
		WithArgs(int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE users SET`).WithArgs(int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repository.DeleteUser(context.Background(), 2); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
