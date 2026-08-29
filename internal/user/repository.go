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

func (r *Repository) FindByLoginIdentifier(
	ctx context.Context,
	identifier string,
) (User, error) {
	const query = `
			SELECT id, name, email, birthday, password_hash, created_at, role, is_active
		FROM users
		WHERE LOWER(email) = LOWER($1) OR name = $1
		ORDER BY id
		LIMIT 1
	`

	var foundUser User

	err := r.db.QueryRowContext(ctx, query, identifier).Scan(
		&foundUser.ID,
		&foundUser.Name,
		&foundUser.Email,
		&foundUser.Birthday,
		&foundUser.PasswordHash,
		&foundUser.CreatedAt,
		&foundUser.Role,
		&foundUser.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, sql.ErrNoRows
		}

		return User{}, fmt.Errorf("find user by login identifier: %w", err)
	}

	return foundUser, nil
}

func (r *Repository) FindByID(ctx context.Context, userID int64) (User, error) {
	const query = `
			SELECT id, name, email, birthday, created_at, bio, location, website, role, is_active
		FROM users
		WHERE id = $1
	`

	var foundUser User
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&foundUser.ID,
		&foundUser.Name,
		&foundUser.Email,
		&foundUser.Birthday,
		&foundUser.CreatedAt,
		&foundUser.Bio,
		&foundUser.Location,
		&foundUser.Website,
		&foundUser.Role,
		&foundUser.IsActive,
	)
	if err != nil {
		return User{}, fmt.Errorf("find user by id: %w", err)
	}

	return foundUser, nil
}

func (r *Repository) CreateSchoolUser(ctx context.Context, name, email, passwordHash, role string) (User, error) {
	const query = `INSERT INTO users (name,email,birthday,password_hash,role)
		VALUES ($1,$2,DATE '2000-01-01',$3,$4)
		RETURNING id,name,email,birthday,created_at,role,is_active`
	var created User
	err := r.db.QueryRowContext(ctx, query, name, email, passwordHash, role).Scan(
		&created.ID, &created.Name, &created.Email, &created.Birthday,
		&created.CreatedAt, &created.Role, &created.IsActive)
	return created, err
}

func (r *Repository) FindByName(ctx context.Context, name string) (User, error) {
	const query = `SELECT id, name, email, birthday, created_at, bio, location, website FROM users WHERE name = $1`
	var found User
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&found.ID, &found.Name, &found.Email, &found.Birthday, &found.CreatedAt,
		&found.Bio, &found.Location, &found.Website,
	)
	if err != nil {
		return User{}, fmt.Errorf("find user by name: %w", err)
	}
	return found, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, userID int64, name, bio, location, website string) (User, error) {
	const query = `
		UPDATE users SET name = $2, bio = $3, location = $4, website = $5
		WHERE id = $1
		RETURNING id, name, email, birthday, created_at, bio, location, website`
	var updated User
	err := r.db.QueryRowContext(ctx, query, userID, name, bio, location, website).Scan(
		&updated.ID, &updated.Name, &updated.Email, &updated.Birthday, &updated.CreatedAt,
		&updated.Bio, &updated.Location, &updated.Website,
	)
	if err != nil {
		return User{}, fmt.Errorf("update user profile: %w", err)
	}
	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, userID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted users: %w", err)
	}
	if deleted == 0 {
		return sql.ErrNoRows
	}
	return nil
}
