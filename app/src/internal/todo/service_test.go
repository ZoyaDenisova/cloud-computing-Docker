package todo

import (
	"context"
	"errors"
	"testing"
	"time"
)

type serviceRepoStub struct {
	listFn   func(ctx context.Context) ([]Task, error)
	getFn    func(ctx context.Context, id int64) (Task, error)
	createFn func(ctx context.Context, input CreateTaskInput) (Task, error)
	updateFn func(ctx context.Context, id int64, input UpdateTaskInput) (Task, error)
	deleteFn func(ctx context.Context, id int64) error
}

func (s *serviceRepoStub) List(ctx context.Context) ([]Task, error) {
	if s.listFn == nil {
		return nil, nil
	}

	return s.listFn(ctx)
}

func (s *serviceRepoStub) GetByID(ctx context.Context, id int64) (Task, error) {
	if s.getFn == nil {
		return Task{}, nil
	}

	return s.getFn(ctx, id)
}

func (s *serviceRepoStub) Create(ctx context.Context, input CreateTaskInput) (Task, error) {
	if s.createFn == nil {
		return Task{}, nil
	}

	return s.createFn(ctx, input)
}

func (s *serviceRepoStub) Update(ctx context.Context, id int64, input UpdateTaskInput) (Task, error) {
	if s.updateFn == nil {
		return Task{}, nil
	}

	return s.updateFn(ctx, id, input)
}

func (s *serviceRepoStub) Delete(ctx context.Context, id int64) error {
	if s.deleteFn == nil {
		return nil
	}

	return s.deleteFn(ctx, id)
}

func TestServiceGetByIDRejectsInvalidID(t *testing.T) {
	svc := NewService(&serviceRepoStub{})

	_, err := svc.GetByID(context.Background(), 0)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestServiceCreateNormalizesInput(t *testing.T) {
	t.Parallel()

	var captured CreateTaskInput
	dueDate := time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC)
	expected := Task{ID: 10, Title: "Task"}

	repo := &serviceRepoStub{
		createFn: func(_ context.Context, input CreateTaskInput) (Task, error) {
			captured = input
			return expected, nil
		},
	}
	svc := NewService(repo)

	got, err := svc.Create(context.Background(), CreateTaskInput{
		Title:       "  Task  ",
		Description: "  Description  ",
		Priority:    3,
		DueDate:     &dueDate,
		Completed:   true,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != expected {
		t.Fatalf("unexpected task returned: %+v", got)
	}

	if captured.Title != "Task" {
		t.Fatalf("expected title to be trimmed, got %q", captured.Title)
	}
	if captured.Description != "Description" {
		t.Fatalf("expected description to be trimmed, got %q", captured.Description)
	}
	if captured.Priority != 3 {
		t.Fatalf("expected priority=3, got %d", captured.Priority)
	}
	if captured.DueDate == nil {
		t.Fatalf("expected due date to be passed to repository")
	}
}

func TestServiceCreateValidationError(t *testing.T) {
	t.Parallel()

	called := false
	repo := &serviceRepoStub{
		createFn: func(_ context.Context, input CreateTaskInput) (Task, error) {
			called = true
			return Task{}, nil
		},
	}
	svc := NewService(repo)

	_, err := svc.Create(context.Background(), CreateTaskInput{
		Title:    "",
		Priority: 1,
	})
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if called {
		t.Fatalf("repository Create must not be called when validation fails")
	}
}

func TestServiceUpdateNormalizesInput(t *testing.T) {
	t.Parallel()

	var capturedID int64
	var capturedInput UpdateTaskInput

	repo := &serviceRepoStub{
		updateFn: func(_ context.Context, id int64, input UpdateTaskInput) (Task, error) {
			capturedID = id
			capturedInput = input
			return Task{ID: id}, nil
		},
	}
	svc := NewService(repo)

	_, err := svc.Update(context.Background(), 7, UpdateTaskInput{
		Title:       "  Updated  ",
		Description: "  Desc  ",
		Priority:    5,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if capturedID != 7 {
		t.Fatalf("expected id=7, got %d", capturedID)
	}
	if capturedInput.Title != "Updated" {
		t.Fatalf("expected trimmed title, got %q", capturedInput.Title)
	}
	if capturedInput.Description != "Desc" {
		t.Fatalf("expected trimmed description, got %q", capturedInput.Description)
	}
}

func TestServiceDeleteRejectsInvalidID(t *testing.T) {
	svc := NewService(&serviceRepoStub{})

	err := svc.Delete(context.Background(), -10)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}
