package todo

import (
	"context"
	"strings"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context) ([]Task, error) {
	return s.repo.List(ctx)
}

func (s *Service) GetByID(ctx context.Context, id int64) (Task, error) {
	if id <= 0 {
		return Task{}, ValidationError{Message: "invalid task id"}
	}

	return s.repo.GetByID(ctx, id)
}

func (s *Service) Create(ctx context.Context, input CreateTaskInput) (Task, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return Task{}, err
	}

	return s.repo.Create(ctx, normalized)
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateTaskInput) (Task, error) {
	if id <= 0 {
		return Task{}, ValidationError{Message: "invalid task id"}
	}

	normalized, err := normalizeUpdateInput(input)
	if err != nil {
		return Task{}, err
	}

	return s.repo.Update(ctx, id, normalized)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ValidationError{Message: "invalid task id"}
	}

	return s.repo.Delete(ctx, id)
}

func normalizeCreateInput(input CreateTaskInput) (CreateTaskInput, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return CreateTaskInput{}, ValidationError{Message: "title is required"}
	}

	if input.Priority < 1 || input.Priority > 5 {
		return CreateTaskInput{}, ValidationError{Message: "priority must be between 1 and 5"}
	}

	input.Title = title
	input.Description = strings.TrimSpace(input.Description)
	return input, nil
}

func normalizeUpdateInput(input UpdateTaskInput) (UpdateTaskInput, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return UpdateTaskInput{}, ValidationError{Message: "title is required"}
	}

	if input.Priority < 1 || input.Priority > 5 {
		return UpdateTaskInput{}, ValidationError{Message: "priority must be between 1 and 5"}
	}

	input.Title = title
	input.Description = strings.TrimSpace(input.Description)
	return input, nil
}
