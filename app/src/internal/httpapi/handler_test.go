package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/todo"
)

type handlerRepoStub struct {
	listFn   func(ctx context.Context) ([]todo.Task, error)
	getFn    func(ctx context.Context, id int64) (todo.Task, error)
	createFn func(ctx context.Context, input todo.CreateTaskInput) (todo.Task, error)
	updateFn func(ctx context.Context, id int64, input todo.UpdateTaskInput) (todo.Task, error)
	deleteFn func(ctx context.Context, id int64) error
}

func (s *handlerRepoStub) List(ctx context.Context) ([]todo.Task, error) {
	if s.listFn == nil {
		return nil, nil
	}

	return s.listFn(ctx)
}

func (s *handlerRepoStub) GetByID(ctx context.Context, id int64) (todo.Task, error) {
	if s.getFn == nil {
		return todo.Task{}, nil
	}

	return s.getFn(ctx, id)
}

func (s *handlerRepoStub) Create(ctx context.Context, input todo.CreateTaskInput) (todo.Task, error) {
	if s.createFn == nil {
		return todo.Task{}, nil
	}

	return s.createFn(ctx, input)
}

func (s *handlerRepoStub) Update(ctx context.Context, id int64, input todo.UpdateTaskInput) (todo.Task, error) {
	if s.updateFn == nil {
		return todo.Task{}, nil
	}

	return s.updateFn(ctx, id, input)
}

func (s *handlerRepoStub) Delete(ctx context.Context, id int64) error {
	if s.deleteFn == nil {
		return nil
	}

	return s.deleteFn(ctx, id)
}

func newTestMux(repo todo.Repository) *http.ServeMux {
	service := todo.NewService(repo)
	handler := NewHandler(service)

	mux := http.NewServeMux()
	handler.Register(mux)

	return mux
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) errorResponse {
	t.Helper()

	var response errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("cannot decode error response: %v", err)
	}

	return response
}

func TestTasksHandlerMethodNotAllowed(t *testing.T) {
	t.Parallel()

	mux := newTestMux(&handlerRepoStub{})
	req := httptest.NewRequest(http.MethodPatch, "/tasks", nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, recorder.Code)
	}

	response := decodeError(t, recorder)
	if response.Error != "method not allowed" {
		t.Fatalf("unexpected error message: %q", response.Error)
	}
}

func TestCreateTaskSuccess(t *testing.T) {
	t.Parallel()

	var captured todo.CreateTaskInput

	repo := &handlerRepoStub{
		createFn: func(_ context.Context, input todo.CreateTaskInput) (todo.Task, error) {
			captured = input
			now := time.Now().UTC()
			return todo.Task{
				ID:          1,
				Title:       input.Title,
				Description: input.Description,
				Priority:    input.Priority,
				Completed:   input.Completed,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	mux := newTestMux(repo)
	body := `{"title":"  Task ","description":"  desc  ","priority":3,"due_date":"2026-03-20","completed":true}`
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.Code)
	}
	if captured.Title != "Task" {
		t.Fatalf("expected trimmed title, got %q", captured.Title)
	}
	if captured.Description != "desc" {
		t.Fatalf("expected trimmed description, got %q", captured.Description)
	}
	if captured.DueDate == nil || captured.DueDate.Format("2006-01-02") != "2026-03-20" {
		t.Fatalf("expected parsed due date, got %+v", captured.DueDate)
	}
}

func TestCreateTaskBadRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{
			name: "unknown field",
			body: `{"title":"Task","priority":3,"unknown":"x"}`,
		},
		{
			name: "bad due date",
			body: `{"title":"Task","priority":3,"due_date":"20-03-2026"}`,
		},
		{
			name: "validation error",
			body: `{"title":" ","priority":3}`,
		},
	}

	mux := newTestMux(&handlerRepoStub{})
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			mux.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
			}
		})
	}
}

func TestGetTaskByIDErrors(t *testing.T) {
	t.Parallel()

	repo := &handlerRepoStub{
		getFn: func(_ context.Context, id int64) (todo.Task, error) {
			switch id {
			case 2:
				return todo.Task{}, todo.ErrNotFound
			case 3:
				return todo.Task{}, context.DeadlineExceeded
			case 4:
				return todo.Task{}, context.Canceled
			default:
				return todo.Task{}, errors.New("db failure")
			}
		},
	}

	mux := newTestMux(repo)

	cases := []struct {
		path       string
		wantStatus int
	}{
		{path: "/tasks/2", wantStatus: http.StatusNotFound},
		{path: "/tasks/3", wantStatus: http.StatusGatewayTimeout},
		{path: "/tasks/4", wantStatus: http.StatusRequestTimeout},
		{path: "/tasks/5", wantStatus: http.StatusInternalServerError},
		{path: "/tasks/abc", wantStatus: http.StatusBadRequest},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, testCase.path, nil)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, req)

			if recorder.Code != testCase.wantStatus {
				t.Fatalf("expected status %d, got %d", testCase.wantStatus, recorder.Code)
			}
		})
	}
}

func TestDeleteTaskSuccess(t *testing.T) {
	t.Parallel()

	called := false
	repo := &handlerRepoStub{
		deleteFn: func(_ context.Context, id int64) error {
			called = true
			if id != 7 {
				t.Fatalf("expected id=7, got %d", id)
			}
			return nil
		},
	}

	mux := newTestMux(repo)
	req := httptest.NewRequest(http.MethodDelete, "/tasks/7", nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
	if !called {
		t.Fatalf("expected delete to be called")
	}
	if strings.TrimSpace(recorder.Body.String()) != "" {
		t.Fatalf("expected empty body for 204, got %q", recorder.Body.String())
	}
}

func TestListTasksFailure(t *testing.T) {
	t.Parallel()

	repo := &handlerRepoStub{
		listFn: func(_ context.Context) ([]todo.Task, error) {
			return nil, errors.New("db down")
		},
	}

	mux := newTestMux(repo)
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}

	response := decodeError(t, recorder)
	if response.Error != "cannot fetch tasks" {
		t.Fatalf("unexpected error message: %q", response.Error)
	}
}

func TestParseTaskID(t *testing.T) {
	t.Parallel()

	id, err := parseTaskID("/tasks/42")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if id != 42 {
		t.Fatalf("expected id=42, got %d", id)
	}

	_, err = parseTaskID("/tasks/42/other")
	if err == nil {
		t.Fatalf("expected error for nested path")
	}
}

func TestParseDueDate(t *testing.T) {
	t.Parallel()

	dateValue := "2026-03-22"
	parsed, err := parseDueDate(&dateValue)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if parsed == nil || parsed.Format("2006-01-02") != "2026-03-22" {
		t.Fatalf("unexpected parsed date: %+v", parsed)
	}

	empty := "   "
	parsed, err = parseDueDate(&empty)
	if err != nil {
		t.Fatalf("expected nil error for blank value, got %v", err)
	}
	if parsed != nil {
		t.Fatalf("expected nil date for blank value")
	}

	invalid := "22/03/2026"
	_, err = parseDueDate(&invalid)
	if err == nil {
		t.Fatalf("expected error for invalid format")
	}
}
