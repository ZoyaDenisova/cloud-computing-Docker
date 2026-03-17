package todo

import "errors"

var ErrNotFound = errors.New("task not found")

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}
