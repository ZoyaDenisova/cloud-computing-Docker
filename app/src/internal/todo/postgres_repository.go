package todo

import (
	"context"
	"database/sql"
	"errors"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) List(ctx context.Context) ([]Task, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, title, description, priority, due_date, completed, created_at, updated_at
		FROM tasks
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (Task, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, title, description, priority, due_date, completed, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`, id)

	task, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}

	return task, nil
}

func (r *PostgresRepository) Create(ctx context.Context, input CreateTaskInput) (Task, error) {
	var dueDate interface{}
	if input.DueDate != nil {
		dueDate = *input.DueDate
	}

	row := r.db.QueryRowContext(ctx, `
		INSERT INTO tasks (title, description, priority, due_date, completed)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, title, description, priority, due_date, completed, created_at, updated_at
	`, input.Title, input.Description, input.Priority, dueDate, input.Completed)

	task, err := scanTask(row)
	if err != nil {
		return Task{}, err
	}

	return task, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id int64, input UpdateTaskInput) (Task, error) {
	var dueDate interface{}
	if input.DueDate != nil {
		dueDate = *input.DueDate
	}

	row := r.db.QueryRowContext(ctx, `
		UPDATE tasks
		SET title = $1, description = $2, priority = $3, due_date = $4, completed = $5, updated_at = NOW()
		WHERE id = $6
		RETURNING id, title, description, priority, due_date, completed, created_at, updated_at
	`, input.Title, input.Description, input.Priority, dueDate, input.Completed, id)

	task, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}

	return task, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = $1", id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func scanTask(scanner interface {
	Scan(dest ...interface{}) error
}) (Task, error) {
	var task Task
	var dueDate sql.NullTime

	err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&task.Priority,
		&dueDate,
		&task.Completed,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return Task{}, err
	}

	if dueDate.Valid {
		formatted := dueDate.Time.Format("2006-01-02")
		task.DueDate = &formatted
	}

	return task, nil
}
