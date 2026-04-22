package todo

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func newPostgresRepoForTest(t *testing.T) (*PostgresRepository, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("cannot create sqlmock: %v", err)
	}

	cleanup := func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
		_ = db.Close()
	}

	return NewPostgresRepository(db), mock, cleanup
}

func TestPostgresRepositoryList(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newPostgresRepoForTest(t)
	defer cleanup()

	dueDate := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 3, 2, 11, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id", "title", "description", "priority", "due_date", "completed", "created_at", "updated_at",
	}).AddRow(1, "Task", "Description", 3, dueDate, false, createdAt, updatedAt)

	mock.ExpectQuery(`SELECT id, title, description, priority, due_date, completed, created_at, updated_at\s+FROM tasks\s+ORDER BY id`).
		WillReturnRows(rows)

	tasks, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].DueDate == nil || *tasks[0].DueDate != "2026-03-20" {
		t.Fatalf("expected due_date=2026-03-20, got %+v", tasks[0].DueDate)
	}
}

func TestPostgresRepositoryGetByIDNotFound(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newPostgresRepoForTest(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT id, title, description, priority, due_date, completed, created_at, updated_at\s+FROM tasks\s+WHERE id = \$1`).
		WithArgs(int64(100)).
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetByID(context.Background(), 100)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresRepositoryCreateWithoutDueDate(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newPostgresRepoForTest(t)
	defer cleanup()

	createdAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 3, 2, 11, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id", "title", "description", "priority", "due_date", "completed", "created_at", "updated_at",
	}).AddRow(2, "New task", "Text", 2, nil, true, createdAt, updatedAt)

	mock.ExpectQuery(`INSERT INTO tasks \(title, description, priority, due_date, completed\)\s+VALUES \(\$1, \$2, \$3, \$4, \$5\)\s+RETURNING id, title, description, priority, due_date, completed, created_at, updated_at`).
		WithArgs("New task", "Text", 2, nil, true).
		WillReturnRows(rows)

	task, err := repo.Create(context.Background(), CreateTaskInput{
		Title:       "New task",
		Description: "Text",
		Priority:    2,
		Completed:   true,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if task.DueDate != nil {
		t.Fatalf("expected nil due_date, got %v", *task.DueDate)
	}
}

func TestPostgresRepositoryUpdateNotFound(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newPostgresRepoForTest(t)
	defer cleanup()

	dueDate := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`UPDATE tasks\s+SET title = \$1, description = \$2, priority = \$3, due_date = \$4, completed = \$5, updated_at = NOW\(\)\s+WHERE id = \$6\s+RETURNING id, title, description, priority, due_date, completed, created_at, updated_at`).
		WithArgs("Updated", "Desc", 4, dueDate, false, int64(99)).
		WillReturnError(sql.ErrNoRows)

	_, err := repo.Update(context.Background(), 99, UpdateTaskInput{
		Title:       "Updated",
		Description: "Desc",
		Priority:    4,
		DueDate:     &dueDate,
		Completed:   false,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresRepositoryDelete(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newPostgresRepoForTest(t)
	defer cleanup()

	mock.ExpectExec(`DELETE FROM tasks WHERE id = \$1`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Delete(context.Background(), 1); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestPostgresRepositoryDeleteNotFound(t *testing.T) {
	t.Parallel()

	repo, mock, cleanup := newPostgresRepoForTest(t)
	defer cleanup()

	mock.ExpectExec(`DELETE FROM tasks WHERE id = \$1`).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Delete(context.Background(), 7)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
