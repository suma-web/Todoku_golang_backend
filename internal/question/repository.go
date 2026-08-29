package question

import (
	"context"
	"database/sql"
)

type Repository interface {
	ListCategories(context.Context) ([]Category, error)
	CreateCategory(context.Context, Category) (Category, error)
	UpdateCategory(context.Context, Category) (Category, error)
	Create(context.Context, int64, Question) (Question, error)
	List(context.Context, int64, string) ([]Question, error)
	CanAccess(context.Context, int64, int64) (bool, error)
	Get(context.Context, int64) (Question, error)
	ListAnswers(context.Context, int64) ([]Answer, error)
	CanAnswer(context.Context, int64, int64) (bool, error)
	CreateAnswer(context.Context, int64, int64, string) (Answer, error)
	Resolve(context.Context, int64, int64) (bool, error)
}

type SQLRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &SQLRepository{db: db}
}

const questionSelect = `SELECT q.id,q.user_id,u.name,q.category_id,c.name,g.name,q.title,q.content,q.visibility,q.status,q.created_at,q.updated_at FROM questions q JOIN users u ON u.id=q.user_id JOIN question_categories c ON c.id=q.category_id JOIN school_groups g ON g.id=c.group_id`

type scanner interface {
	Scan(...any) error
}

func scanQuestion(row scanner) (Question, error) {
	var item Question
	err := row.Scan(
		&item.ID, &item.UserID, &item.UserName, &item.CategoryID,
		&item.CategoryName, &item.DepartmentName, &item.Title, &item.Content,
		&item.Visibility, &item.Status, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func (r *SQLRepository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id,c.name,c.group_id,g.name,c.is_active FROM question_categories c JOIN school_groups g ON g.id=c.group_id ORDER BY c.is_active DESC,c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Category{}
	for rows.Next() {
		var item Category
		if err := rows.Scan(&item.ID, &item.Name, &item.GroupID, &item.GroupName, &item.IsActive); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) CreateCategory(ctx context.Context, input Category) (Category, error) {
	err := r.db.QueryRowContext(ctx, `INSERT INTO question_categories(name,group_id) VALUES($1,$2) RETURNING id`, input.Name, input.GroupID).Scan(&input.ID)
	if err == nil {
		err = r.db.QueryRowContext(ctx, `SELECT name FROM school_groups WHERE id=$1`, input.GroupID).Scan(&input.GroupName)
	}
	input.IsActive = true
	return input, err
}

func (r *SQLRepository) UpdateCategory(ctx context.Context, input Category) (Category, error) {
	err := r.db.QueryRowContext(ctx, `UPDATE question_categories SET name=$2,group_id=$3,is_active=$4 WHERE id=$1 RETURNING id`, input.ID, input.Name, input.GroupID, input.IsActive).Scan(&input.ID)
	if err != nil {
		return Category{}, err
	}
	err = r.db.QueryRowContext(ctx, `SELECT name FROM school_groups WHERE id=$1`, input.GroupID).Scan(&input.GroupName)
	return input, err
}

func (r *SQLRepository) Create(ctx context.Context, userID int64, input Question) (Question, error) {
	err := r.db.QueryRowContext(ctx, `INSERT INTO questions(user_id,category_id,title,content,visibility) SELECT $1,$2,$3,$4,$5 WHERE EXISTS(SELECT 1 FROM question_categories WHERE id=$2 AND is_active) RETURNING id,status,created_at,updated_at`,
		userID, input.CategoryID, input.Title, input.Content, input.Visibility,
	).Scan(&input.ID, &input.Status, &input.CreatedAt, &input.UpdatedAt)
	input.UserID = userID
	return input, err
}

func (r *SQLRepository) List(ctx context.Context, userID int64, status string) ([]Question, error) {
	query := questionSelect + ` WHERE (q.user_id=$1 OR q.visibility='public' OR EXISTS(SELECT 1 FROM user_school_groups ug JOIN users viewer ON viewer.id=ug.user_id WHERE ug.user_id=$1 AND ug.group_id=c.group_id AND viewer.role IN('teacher','admin')) OR (SELECT role FROM users WHERE id=$1)='admin') AND ($2='' OR q.status=$2) ORDER BY q.updated_at DESC LIMIT 100`
	rows, err := r.db.QueryContext(ctx, query, userID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Question{}
	for rows.Next() {
		item, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) CanAccess(ctx context.Context, questionID, userID int64) (bool, error) {
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM questions q JOIN question_categories c ON c.id=q.category_id JOIN users viewer ON viewer.id=$2 WHERE q.id=$1 AND(q.user_id=$2 OR q.visibility='public' OR viewer.role='admin' OR(viewer.role='teacher' AND EXISTS(SELECT 1 FROM user_school_groups ug WHERE ug.user_id=$2 AND ug.group_id=c.group_id))))`, questionID, userID).Scan(&allowed)
	return allowed, err
}

func (r *SQLRepository) Get(ctx context.Context, questionID int64) (Question, error) {
	return scanQuestion(r.db.QueryRowContext(ctx, questionSelect+` WHERE q.id=$1`, questionID))
}

func (r *SQLRepository) ListAnswers(ctx context.Context, questionID int64) ([]Answer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT a.id,a.question_id,a.user_id,u.name,a.content,a.created_at FROM question_answers a JOIN users u ON u.id=a.user_id WHERE a.question_id=$1 ORDER BY a.created_at,a.id`, questionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Answer{}
	for rows.Next() {
		var item Answer
		if err := rows.Scan(&item.ID, &item.QuestionID, &item.UserID, &item.UserName, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) CanAnswer(ctx context.Context, questionID, userID int64) (bool, error) {
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM questions q JOIN question_categories c ON c.id=q.category_id JOIN users u ON u.id=$2 WHERE q.id=$1 AND(u.role='admin' OR(u.role='teacher' AND EXISTS(SELECT 1 FROM user_school_groups ug WHERE ug.user_id=$2 AND ug.group_id=c.group_id))))`, questionID, userID).Scan(&allowed)
	return allowed, err
}

func (r *SQLRepository) CreateAnswer(ctx context.Context, questionID, userID int64, content string) (Answer, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Answer{}, err
	}
	defer tx.Rollback()

	item := Answer{QuestionID: questionID, UserID: userID, Content: content}
	err = tx.QueryRowContext(ctx, `INSERT INTO question_answers(question_id,user_id,content) VALUES($1,$2,$3) RETURNING id,created_at`, questionID, userID, content).Scan(&item.ID, &item.CreatedAt)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE questions SET status='answered',updated_at=NOW() WHERE id=$1 AND status<>'resolved'`, questionID)
	}
	if err != nil {
		return Answer{}, err
	}
	if err := tx.Commit(); err != nil {
		return Answer{}, err
	}
	return item, nil
}

func (r *SQLRepository) Resolve(ctx context.Context, questionID, userID int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE questions SET status='resolved',updated_at=NOW() WHERE id=$1 AND user_id=$2`, questionID, userID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
