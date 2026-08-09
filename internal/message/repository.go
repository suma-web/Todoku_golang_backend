package message

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrUserNotFound = errors.New("group member not found")

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateGroup(ctx context.Context, creatorID int64, name string, memberNames []string) (Group, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Group{}, fmt.Errorf("begin create group: %w", err)
	}
	defer tx.Rollback()
	var group Group
	err = tx.QueryRowContext(ctx, `
		INSERT INTO message_groups (name, created_by) VALUES ($1, $2)
		RETURNING id, name, created_by, created_at`, name, creatorID).Scan(
		&group.ID, &group.Name, &group.CreatedBy, &group.CreatedAt,
	)
	if err != nil {
		return Group{}, fmt.Errorf("create group: %w", err)
	}
	const addMember = `INSERT INTO message_group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	if _, err := tx.ExecContext(ctx, addMember, group.ID, creatorID); err != nil {
		return Group{}, fmt.Errorf("add creator: %w", err)
	}
	for _, userName := range memberNames {
		var userID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE name = $1 ORDER BY id LIMIT 1`, userName).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			return Group{}, ErrUserNotFound
		}
		if err != nil {
			return Group{}, fmt.Errorf("find member: %w", err)
		}
		if _, err := tx.ExecContext(ctx, addMember, group.ID, userID); err != nil {
			return Group{}, fmt.Errorf("add member: %w", err)
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM message_group_members WHERE group_id = $1`, group.ID).Scan(&group.MemberCount); err != nil {
		return Group{}, fmt.Errorf("count members: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Group{}, fmt.Errorf("commit group: %w", err)
	}
	return group, nil
}

const groupSelect = `
	SELECT groups.id, groups.name, groups.created_by, groups.created_at,
	       (SELECT COUNT(*) FROM message_group_members members WHERE members.group_id = groups.id),
	       latest.message, latest.created_at
	FROM message_groups groups
	LEFT JOIN LATERAL (
		SELECT message, created_at FROM direct_messages
		WHERE group_id = groups.id ORDER BY created_at DESC, id DESC LIMIT 1
	) latest ON TRUE`

func scanGroup(scanner interface{ Scan(...any) error }) (Group, error) {
	var group Group
	err := scanner.Scan(&group.ID, &group.Name, &group.CreatedBy, &group.CreatedAt, &group.MemberCount, &group.LastMessage, &group.LastMessageAt)
	return group, err
}

func (r *Repository) ListGroups(ctx context.Context, userID int64, limit, offset int) ([]Group, error) {
	query := groupSelect + `
		WHERE EXISTS (SELECT 1 FROM message_group_members mine WHERE mine.group_id = groups.id AND mine.user_id = $1)
		ORDER BY COALESCE(latest.created_at, groups.created_at) DESC, groups.id DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()
	groups := make([]Group, 0)
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate groups: %w", err)
	}
	return groups, nil
}

func (r *Repository) FindGroup(ctx context.Context, groupID, userID int64) (Group, error) {
	query := groupSelect + `
		WHERE groups.id = $1 AND EXISTS (
			SELECT 1 FROM message_group_members mine WHERE mine.group_id = groups.id AND mine.user_id = $2
		)`
	group, err := scanGroup(r.db.QueryRowContext(ctx, query, groupID, userID))
	if err != nil {
		return Group{}, fmt.Errorf("find group: %w", err)
	}
	return group, nil
}

func (r *Repository) IsMember(ctx context.Context, groupID, userID int64) (bool, error) {
	var member bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM message_group_members WHERE group_id = $1 AND user_id = $2)`, groupID, userID).Scan(&member)
	if err != nil {
		return false, fmt.Errorf("check member: %w", err)
	}
	return member, nil
}

func (r *Repository) CreateMessage(ctx context.Context, groupID, userID int64, text string) (Message, error) {
	const query = `
		WITH created AS (
			INSERT INTO direct_messages (group_id, user_id, message) VALUES ($1, $2, $3)
			RETURNING id, group_id, user_id, message, created_at
		)
		SELECT created.id, created.group_id, created.user_id, users.name, created.message, created.created_at
		FROM created JOIN users ON users.id = created.user_id`
	var item Message
	err := r.db.QueryRowContext(ctx, query, groupID, userID, text).Scan(&item.ID, &item.GroupID, &item.UserID, &item.UserName, &item.Message, &item.CreatedAt)
	if err != nil {
		return Message{}, fmt.Errorf("create message: %w", err)
	}
	return item, nil
}

func (r *Repository) ListMessages(ctx context.Context, groupID int64, limit, offset int) ([]Message, error) {
	const query = `
		SELECT recent.id, recent.group_id, recent.user_id, recent.name, recent.message, recent.created_at
		FROM (
			SELECT messages.id, messages.group_id, messages.user_id, users.name, messages.message, messages.created_at
			FROM direct_messages messages JOIN users ON users.id = messages.user_id
			WHERE messages.group_id = $1 ORDER BY messages.created_at DESC, messages.id DESC LIMIT $2 OFFSET $3
		) recent ORDER BY recent.created_at ASC, recent.id ASC`
	rows, err := r.db.QueryContext(ctx, query, groupID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	items := make([]Message, 0)
	for rows.Next() {
		var item Message
		if err := rows.Scan(&item.ID, &item.GroupID, &item.UserID, &item.UserName, &item.Message, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}
	return items, nil
}
