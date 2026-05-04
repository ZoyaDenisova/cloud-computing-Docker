package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/segmentio/kafka-go"
)

const (
	reportTopPosts       = "top_posts_by_comments"
	reportPostsByDay     = "posts_by_day"
	reportCommentsByDay  = "comments_by_day"
	defaultRetryAttempts = 30
)

type config struct {
	appPort      string
	dbHost       string
	dbPort       string
	dbUser       string
	dbPassword   string
	dbName       string
	kafkaBrokers []string
	kafkaTopic   string
}

type store struct {
	db *sql.DB
}

type server struct {
	store *store
}

type postData struct {
	Author    string `json:"author"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at,omitempty"`
}

type commentData struct {
	PostID    int64  `json:"post_id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at,omitempty"`
}

type dataMessage struct {
	Type    string       `json:"type"`
	Post    *postData    `json:"post,omitempty"`
	Comment *commentData `json:"comment,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type searchItem struct {
	PostID        int64     `json:"post_id"`
	Author        string    `json:"author"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	CreatedAt     time.Time `json:"created_at"`
	CommentsCount int64     `json:"comments_count"`
}

type topPostReportItem struct {
	PostID        int64  `json:"post_id"`
	Title         string `json:"title"`
	CommentsCount int64  `json:"comments_count"`
}

type dailyReportItem struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type existsResponse struct {
	Exists bool `json:"exists"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	db, err := connectWithRetry(context.Background(), cfg.dsn(), defaultRetryAttempts, 2*time.Second)
	if err != nil {
		log.Fatalf("cannot connect to db: %v", err)
	}
	defer db.Close()

	st := &store{db: db}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.kafkaBrokers,
		Topic:       cfg.kafkaTopic,
		Partition:   0,
		StartOffset: kafka.FirstOffset,
		MinBytes:    1,
		MaxBytes:    10e6,
	})
	defer reader.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runConsumer(ctx, reader, st)

	srv := &server{store: st}
	mux := http.NewServeMux()
	mux.HandleFunc("/search", srv.handleSearch)
	mux.HandleFunc("/reports", srv.handleReports)
	mux.HandleFunc("/posts/", srv.handlePostExists)

	httpServer := &http.Server{
		Addr:         ":" + cfg.appPort,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("data-service started on port %s", cfg.appPort)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("data-service stopped: %v", err)
	}
}

func loadConfig() (config, error) {
	cfg := config{
		appPort:      getEnv("APP_PORT", "8081"),
		dbHost:       getEnv("DB_HOST", "db"),
		dbPort:       getEnv("DB_PORT", "5432"),
		dbUser:       strings.TrimSpace(os.Getenv("DB_USER")),
		dbPassword:   strings.TrimSpace(os.Getenv("DB_PASSWORD")),
		dbName:       strings.TrimSpace(os.Getenv("DB_NAME")),
		kafkaBrokers: splitCSV(getEnv("KAFKA_BROKERS", "kafka:9092")),
		kafkaTopic:   getEnv("KAFKA_TOPIC", "social_data"),
	}

	if cfg.dbUser == "" || cfg.dbPassword == "" || cfg.dbName == "" {
		return config{}, errors.New("DB_USER, DB_PASSWORD and DB_NAME must be set")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (c config) dsn() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.dbHost,
		c.dbPort,
		c.dbUser,
		c.dbPassword,
		c.dbName,
	)
}

func connectWithRetry(ctx context.Context, dsn string, attempts int, delay time.Duration) (*sql.DB, error) {
	var lastErr error

	for i := 1; i <= attempts; i++ {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			lastErr = err
			log.Printf("db open attempt %d/%d failed: %v", i, attempts, err)
		} else {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = db.PingContext(pingCtx)
			cancel()
			if err == nil {
				return db, nil
			}
			lastErr = err
			_ = db.Close()
			log.Printf("db ping attempt %d/%d failed: %v", i, attempts, err)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, lastErr
}

func runConsumer(ctx context.Context, reader *kafka.Reader, st *store) {
	for {
		message, err := reader.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("kafka read failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err := processKafkaMessage(ctx, st, message.Value); err != nil {
			log.Printf("kafka message rejected: %v", err)
		}
	}
}

func processKafkaMessage(ctx context.Context, st *store, raw []byte) error {
	var msg dataMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}

	switch msg.Type {
	case "post":
		if msg.Post == nil {
			return errors.New("post payload is required")
		}
		if strings.TrimSpace(msg.Post.Author) == "" || strings.TrimSpace(msg.Post.Title) == "" || strings.TrimSpace(msg.Post.Body) == "" {
			return errors.New("post.author, post.title and post.body are required")
		}

		createdAt, err := parseCreatedAt(msg.Post.CreatedAt)
		if err != nil {
			return err
		}

		return st.insertPost(ctx, *msg.Post, createdAt)
	case "comment":
		if msg.Comment == nil {
			return errors.New("comment payload is required")
		}
		if msg.Comment.PostID <= 0 || strings.TrimSpace(msg.Comment.Author) == "" || strings.TrimSpace(msg.Comment.Text) == "" {
			return errors.New("comment.post_id, comment.author and comment.text are required")
		}

		createdAt, err := parseCreatedAt(msg.Comment.CreatedAt)
		if err != nil {
			return err
		}

		return st.insertComment(ctx, *msg.Comment, createdAt)
	default:
		return errors.New("unknown message type")
	}
}

func parseCreatedAt(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, errors.New("created_at must be RFC3339")
	}

	return &parsed, nil
}

func (st *store) insertPost(ctx context.Context, post postData, createdAt *time.Time) error {
	query := `
		INSERT INTO posts (author, title, body, created_at)
		VALUES ($1, $2, $3, COALESCE($4, NOW()))
	`
	_, err := st.db.ExecContext(ctx, query, post.Author, post.Title, post.Body, createdAt)
	return err
}

func (st *store) insertComment(ctx context.Context, comment commentData, createdAt *time.Time) error {
	query := `
		INSERT INTO comments (post_id, author, text, created_at)
		VALUES ($1, $2, $3, COALESCE($4, NOW()))
	`
	_, err := st.db.ExecContext(ctx, query, comment.PostID, comment.Author, comment.Text, createdAt)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && string(pgErr.Code) == "23503" {
			return fmt.Errorf("post with id=%d does not exist", comment.PostID)
		}
	}
	return err
}

func (s *server) handlePostExists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	postID, err := parsePostIDFromExistsPath(r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid post id"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	exists, err := s.store.postExists(ctx, postID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "cannot check post"})
		return
	}

	writeJSON(w, http.StatusOK, existsResponse{Exists: exists})
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

func (st *store) postExists(ctx context.Context, postID int64) (bool, error) {
	var exists bool
	err := st.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM posts WHERE id = $1)", postID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	queryValue := strings.TrimSpace(r.URL.Query().Get("q"))

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	items, err := s.store.searchPosts(ctx, queryValue)
	if err != nil {
		log.Printf("search failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "cannot execute search"})
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func (st *store) searchPosts(ctx context.Context, queryValue string) ([]searchItem, error) {
	var (
		query string
		rows  *sql.Rows
		err   error
	)

	base := `
		SELECT p.id, p.author, p.title, p.body, p.created_at, COUNT(c.id) AS comments_count
		FROM posts p
		LEFT JOIN comments c ON c.post_id = p.id
	`

	if queryValue == "" {
		query = base + `
		GROUP BY p.id
		ORDER BY p.id
	`
		rows, err = st.db.QueryContext(ctx, query)
	} else {
		query = base + `
		WHERE p.title ILIKE '%' || $1 || '%'
		   OR p.body ILIKE '%' || $1 || '%'
		   OR p.author ILIKE '%' || $1 || '%'
		   OR EXISTS (
				SELECT 1
				FROM comments c2
				WHERE c2.post_id = p.id
				  AND (c2.text ILIKE '%' || $1 || '%' OR c2.author ILIKE '%' || $1 || '%')
			)
		GROUP BY p.id
		ORDER BY p.id
	`
		rows, err = st.db.QueryContext(ctx, query, queryValue)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]searchItem, 0)
	for rows.Next() {
		var item searchItem
		if err := rows.Scan(&item.PostID, &item.Author, &item.Title, &item.Body, &item.CreatedAt, &item.CommentsCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *server) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	reportType := strings.TrimSpace(r.URL.Query().Get("type"))
	if reportType == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "type is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch reportType {
	case reportTopPosts:
		report, err := s.store.reportTopPostsByComments(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "cannot build report"})
			return
		}
		writeJSON(w, http.StatusOK, report)
	case reportPostsByDay:
		report, err := s.store.reportPostsByDay(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "cannot build report"})
			return
		}
		writeJSON(w, http.StatusOK, report)
	case reportCommentsByDay:
		report, err := s.store.reportCommentsByDay(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "cannot build report"})
			return
		}
		writeJSON(w, http.StatusOK, report)
	default:
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error: "unknown report type, use: top_posts_by_comments, posts_by_day, comments_by_day",
		})
	}
}

func (st *store) reportTopPostsByComments(ctx context.Context) ([]topPostReportItem, error) {
	query := `
		SELECT p.id, p.title, COUNT(c.id) AS comments_count
		FROM posts p
		LEFT JOIN comments c ON c.post_id = p.id
		GROUP BY p.id
		ORDER BY comments_count DESC, p.id
		LIMIT 10
	`
	rows, err := st.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]topPostReportItem, 0)
	for rows.Next() {
		var item topPostReportItem
		if err := rows.Scan(&item.PostID, &item.Title, &item.CommentsCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (st *store) reportPostsByDay(ctx context.Context) ([]dailyReportItem, error) {
	query := `
		SELECT DATE(created_at) AS day, COUNT(*) AS total
		FROM posts
		GROUP BY DATE(created_at)
		ORDER BY day
	`
	return st.runDailyReport(ctx, query)
}

func (st *store) reportCommentsByDay(ctx context.Context) ([]dailyReportItem, error) {
	query := `
		SELECT DATE(created_at) AS day, COUNT(*) AS total
		FROM comments
		GROUP BY DATE(created_at)
		ORDER BY day
	`
	return st.runDailyReport(ctx, query)
}

func (st *store) runDailyReport(ctx context.Context, query string) ([]dailyReportItem, error) {
	rows, err := st.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]dailyReportItem, 0)
	for rows.Next() {
		var (
			day   time.Time
			count int64
		)
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		result = append(result, dailyReportItem{
			Day:   day.Format("2006-01-02"),
			Count: count,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
