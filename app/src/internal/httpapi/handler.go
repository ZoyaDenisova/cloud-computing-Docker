package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"app/internal/todo"
)

type Handler struct {
	service *todo.Service
}

func NewHandler(service *todo.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/tasks", h.tasksHandler)
	mux.HandleFunc("/tasks/", h.taskByIDHandler)
}

func (h *Handler) tasksHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTasks(w, r)
	case http.MethodPost:
		h.createTask(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (h *Handler) taskByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parseTaskID(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid task id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getTaskByID(w, r, id)
	case http.MethodPut:
		h.updateTask(w, r, id)
	case http.MethodDelete:
		h.deleteTask(w, r, id)
	default:
		methodNotAllowed(w)
	}
}

type taskRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    int     `json:"priority"`
	DueDate     *string `json:"due_date"`
	Completed   bool    `json:"completed"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tasks, err := h.service.List(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "cannot fetch tasks"})
		return
	}

	writeJSON(w, http.StatusOK, tasks)
}

func (h *Handler) getTaskByID(w http.ResponseWriter, r *http.Request, id int64) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	task, err := h.service.GetByID(ctx, id)
	if err != nil {
		h.handleServiceError(w, err, "cannot fetch task")
		return
	}

	writeJSON(w, http.StatusOK, task)
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	req, err := decodeTaskRequest(w, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	dueDate, err := parseDueDate(req.DueDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	task, err := h.service.Create(ctx, todo.CreateTaskInput{
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		DueDate:     dueDate,
		Completed:   req.Completed,
	})
	if err != nil {
		h.handleServiceError(w, err, "cannot create task")
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

func (h *Handler) updateTask(w http.ResponseWriter, r *http.Request, id int64) {
	req, err := decodeTaskRequest(w, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	dueDate, err := parseDueDate(req.DueDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	task, err := h.service.Update(ctx, id, todo.UpdateTaskInput{
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		DueDate:     dueDate,
		Completed:   req.Completed,
	})
	if err != nil {
		h.handleServiceError(w, err, "cannot update task")
		return
	}

	writeJSON(w, http.StatusOK, task)
}

func (h *Handler) deleteTask(w http.ResponseWriter, r *http.Request, id int64) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.service.Delete(ctx, id); err != nil {
		h.handleServiceError(w, err, "cannot delete task")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleServiceError(w http.ResponseWriter, err error, defaultMessage string) {
	if errors.Is(err, context.DeadlineExceeded) {
		writeJSON(w, http.StatusGatewayTimeout, errorResponse{Error: "request timed out"})
		return
	}

	if errors.Is(err, context.Canceled) {
		writeJSON(w, http.StatusRequestTimeout, errorResponse{Error: "request was canceled"})
		return
	}

	var validationErr todo.ValidationError
	if errors.As(err, &validationErr) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: validationErr.Error()})
		return
	}

	if errors.Is(err, todo.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "task not found"})
		return
	}

	writeJSON(w, http.StatusInternalServerError, errorResponse{Error: defaultMessage})
}

func decodeTaskRequest(w http.ResponseWriter, r *http.Request) (taskRequest, error) {
	var req taskRequest

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		return taskRequest{}, errors.New("invalid json")
	}

	return req, nil
}

func parseTaskID(path string) (int64, error) {
	idPart := strings.TrimPrefix(path, "/tasks/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, errors.New("invalid id")
	}

	id, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}

	return id, nil
}

func parseDueDate(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}

	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, errors.New("due_date must be in YYYY-MM-DD format")
	}

	return &parsed, nil
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("cannot write response: %v", err)
	}
}
