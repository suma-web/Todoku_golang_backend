package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByEmail(
	ctx context.Context,
	email string,
) (User, error) {
	const query = `
			SELECT id, name, email, password_hash, created_at, role, is_active
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`

	var foundUser User

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&foundUser.ID,
		&foundUser.Name,
		&foundUser.Email,
		&foundUser.PasswordHash,
		&foundUser.CreatedAt,
		&foundUser.Role,
		&foundUser.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, sql.ErrNoRows
		}

		return User{}, fmt.Errorf("find user by email: %w", err)
	}

	return foundUser, nil
}

func (r *Repository) FindByID(ctx context.Context, userID int64) (User, error) {
	const query = `
			SELECT id, name, email, created_at, role, is_active
		FROM users
		WHERE id = $1
	`

	var foundUser User
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&foundUser.ID,
		&foundUser.Name,
		&foundUser.Email,
		&foundUser.CreatedAt,
		&foundUser.Role,
		&foundUser.IsActive,
	)
	if err != nil {
		return User{}, fmt.Errorf("find user by id: %w", err)
	}

	return foundUser, nil
}

func (r *Repository) CreateSchoolUser(ctx context.Context, name, email, passwordHash, role string) (User, error) {
	const query = `INSERT INTO users (name,email,password_hash,role)
		VALUES ($1,$2,$3,$4)
		RETURNING id,name,email,created_at,role,is_active`
	var created User
	err := r.db.QueryRowContext(ctx, query, name, email, passwordHash, role).Scan(
		&created.ID, &created.Name, &created.Email, &created.CreatedAt,
		&created.Role, &created.IsActive)
	return created, err
}
