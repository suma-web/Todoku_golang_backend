package schooladmin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Repository interface {
	ListUsers(context.Context) ([]User, error)
	UpdateUser(context.Context, int64, string, bool) (User, error)
	DeleteUser(context.Context, int64) error
}

type SQLRepository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return &SQLRepository{db: db} }

func (r *SQLRepository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,email,role,is_active FROM users WHERE deleted_at IS NULL ORDER BY role,name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []User{}
	for rows.Next() {
		var item User
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Role, &item.IsActive); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) UpdateUser(ctx context.Context, id int64, role string, active bool) (User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	currentRole, currentActive, err := lockUserAndAdmins(ctx, tx, id)
	if err != nil {
		return User{}, err
	}
	if currentRole == "admin" && currentActive && (role != "admin" || !active) {
		count, err := activeAdminCount(ctx, tx)
		if err != nil {
			return User{}, err
		}
		if count <= 1 {
			return User{}, ErrLastActiveAdmin
		}
	}

	var item User
	err = tx.QueryRowContext(ctx, `UPDATE users SET role=$2,is_active=$3,updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL RETURNING id,name,email,role,is_active`, id, role, active).Scan(
		&item.ID, &item.Name, &item.Email, &item.Role, &item.IsActive,
	)
	if err != nil {
		return User{}, err
	}
	return item, tx.Commit()
}

func (r *SQLRepository) DeleteUser(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	role, active, err := lockUserAndAdmins(ctx, tx, id)
	if err != nil {
		return err
	}
	if role == "admin" && active {
		count, err := activeAdminCount(ctx, tx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastActiveAdmin
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_school_groups WHERE user_id=$1`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET
		name='削除済みユーザー',
		email='deleted-' || id || '@deleted.todoku.invalid',
		password_hash='deleted',
		is_active=FALSE,
		deleted_at=NOW(),
		updated_at=NOW()
		WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func lockUserAndAdmins(ctx context.Context, tx *sql.Tx, id int64) (string, bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM users WHERE role='admin' AND is_active=TRUE AND deleted_at IS NULL ORDER BY id FOR UPDATE`)
	if err != nil {
		return "", false, err
	}
	for rows.Next() {
		var adminID int64
		if err := rows.Scan(&adminID); err != nil {
			rows.Close()
			return "", false, err
		}
	}
	if err := rows.Close(); err != nil {
		return "", false, err
	}
	var role string
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT role,is_active FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&role, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, sql.ErrNoRows
	}
	if err != nil {
		return "", false, fmt.Errorf("lock user: %w", err)
	}
	return role, active, nil
}

func activeAdminCount(ctx context.Context, tx *sql.Tx) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role='admin' AND is_active=TRUE AND deleted_at IS NULL`).Scan(&count)
	return count, err
}
