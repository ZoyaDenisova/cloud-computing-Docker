package todo

import "context"

type Repository interface {
	List(ctx context.Context) ([]Task, error)
	GetByID(ctx context.Context, id int64) (Task, error)
	Create(ctx context.Context, input CreateTaskInput) (Task, error)
	Update(ctx context.Context, id int64, input UpdateTaskInput) (Task, error)
	Delete(ctx context.Context, id int64) error
}
