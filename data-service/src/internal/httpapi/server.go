package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"data-service/internal/model"
)

const (
	reportTopPosts      = "top_posts_by_comments"
	reportPostsByDay    = "posts_by_day"
	reportCommentsByDay = "comments_by_day"
)

type Server struct {
	store DataStore
}

type DataStore interface {
	PostExists(ctx context.Context, postID int64) (bool, error)
	SearchPosts(ctx context.Context, queryValue string) ([]model.SearchItem, error)
	ReportTopPostsByComments(ctx context.Context) ([]model.TopPostReportItem, error)
	ReportPostsByDay(ctx context.Context) ([]model.DailyReportItem, error)
	ReportCommentsByDay(ctx context.Context) ([]model.DailyReportItem, error)
}

func New(st DataStore) *Server {
	return &Server{store: st}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/reports", s.handleReports)
	mux.HandleFunc("/posts/", s.handlePostExists)
	return LoggingMiddleware(mux)
}

func (s *Server) handlePostExists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	postID, err := parsePostIDFromExistsPath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid post id"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	exists, err := s.store.PostExists(ctx, postID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "cannot check post"})
		return
	}

	writeJSON(w, http.StatusOK, model.ExistsResponse{Exists: exists})
}

func parsePostIDFromExistsPath(path string) (int64, error) {
	if !strings.HasPrefix(path, "/posts/") || !strings.HasSuffix(path, "/exists") {
		return 0, errors.New("invalid path")
	}

	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/posts/"), "/exists")
	idPart = strings.TrimSpace(idPart)
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, errors.New("invalid id")
	}

	postID, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || postID <= 0 {
		return 0, errors.New("invalid id")
	}

	return postID, nil
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	queryValue := strings.TrimSpace(r.URL.Query().Get("q"))

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	items, err := s.store.SearchPosts(ctx, queryValue)
	if err != nil {
		log.Printf("search failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "cannot execute search"})
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	reportType := strings.TrimSpace(r.URL.Query().Get("type"))
	if reportType == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "type is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch reportType {
	case reportTopPosts:
		report, err := s.store.ReportTopPostsByComments(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "cannot build report"})
			return
		}
		writeJSON(w, http.StatusOK, report)
	case reportPostsByDay:
		report, err := s.store.ReportPostsByDay(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "cannot build report"})
			return
		}
		writeJSON(w, http.StatusOK, report)
	case reportCommentsByDay:
		report, err := s.store.ReportCommentsByDay(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "cannot build report"})
			return
		}
		writeJSON(w, http.StatusOK, report)
	default:
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: "unknown report type, use: top_posts_by_comments, posts_by_day, comments_by_day",
		})
	}
}
