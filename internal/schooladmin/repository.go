package schooladmin

import (
	"context"
	"database/sql"
)

type Repository interface {
	ListUsers(context.Context) ([]User, error)
	UpdateUser(context.Context, int64, string, bool) (User, error)
}

type SQLRepository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return &SQLRepository{db: db} }

func (r *SQLRepository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,email,role,is_active FROM users ORDER BY role,name,id`)
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
	var item User
	err := r.db.QueryRowContext(ctx, `UPDATE users SET role=$2,is_active=$3,updated_at=NOW() WHERE id=$1 RETURNING id,name,email,role,is_active`, id, role, active).Scan(
		&item.ID, &item.Name, &item.Email, &item.Role, &item.IsActive,
	)
	return item, err
}
