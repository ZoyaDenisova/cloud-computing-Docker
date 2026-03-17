package todo

import "time"

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	DueDate     *string   `json:"due_date,omitempty"`
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateTaskInput struct {
	Title       string
	Description string
	Priority    int
	DueDate     *time.Time
	Completed   bool
}

type UpdateTaskInput struct {
	Title       string
	Description string
	Priority    int
	DueDate     *time.Time
	Completed   bool
}
