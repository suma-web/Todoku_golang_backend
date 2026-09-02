package schoolgroup

import (
	"context"
	"database/sql"
)

type Repository interface {
	List(context.Context) ([]Group, error)
	Create(context.Context, Group) (Group, error)
	CanViewUserGroups(context.Context, int64, int64) (bool, error)
	UserGroups(context.Context, int64) ([]Group, error)
	Members(context.Context, int64) ([]Member, error)
	AddMember(context.Context, int64, int64) error
	RemoveMember(context.Context, int64, int64) (bool, error)
	Delete(context.Context, int64) (bool, error)
}

type SQLRepository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return &SQLRepository{db: db} }

func (r *SQLRepository) List(ctx context.Context) ([]Group, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,type FROM school_groups ORDER BY type,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Group{}
	for rows.Next() {
		var item Group
		if err := rows.Scan(&item.ID, &item.Name, &item.Type); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) Create(ctx context.Context, input Group) (Group, error) {
	err := r.db.QueryRowContext(ctx, `INSERT INTO school_groups(name,type) VALUES($1,$2) RETURNING id`, input.Name, input.Type).Scan(&input.ID)
	return input, err
}

func (r *SQLRepository) CanViewUserGroups(ctx context.Context, viewerID, targetID int64) (bool, error) {
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM users viewer JOIN users target_user ON target_user.id=$2 WHERE viewer.id=$1 AND (
			viewer.id=target_user.id OR viewer.role='admin' OR (viewer.role='teacher' AND EXISTS(
				SELECT 1 FROM user_school_groups viewer_group JOIN user_school_groups target_group ON target_group.group_id=viewer_group.group_id
				WHERE viewer_group.user_id=viewer.id AND target_group.user_id=target_user.id
			))
		))`, viewerID, targetID).Scan(&allowed)
	return allowed, err
}

func (r *SQLRepository) UserGroups(ctx context.Context, userID int64) ([]Group, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT g.id,g.name,g.type FROM school_groups g JOIN user_school_groups ug ON ug.group_id=g.id WHERE ug.user_id=$1 ORDER BY g.type,g.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Group{}
	for rows.Next() {
		var item Group
		if err := rows.Scan(&item.ID, &item.Name, &item.Type); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) Members(ctx context.Context, groupID int64) ([]Member, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT u.id,u.name,u.email,u.role FROM users u JOIN user_school_groups ug ON ug.user_id=u.id WHERE ug.group_id=$1 ORDER BY u.name`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Member{}
	for rows.Next() {
		var item Member
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Role); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) AddMember(ctx context.Context, groupID, userID int64) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO user_school_groups(user_id,group_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, userID, groupID)
	return err
}

func (r *SQLRepository) RemoveMember(ctx context.Context, groupID, userID int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM user_school_groups WHERE group_id=$1 AND user_id=$2`, groupID, userID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

func (r *SQLRepository) Delete(ctx context.Context, groupID int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM school_groups WHERE id=$1`, groupID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
