package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"api-service/internal/apiclient"
	"github.com/segmentio/kafka-go"
)

type config struct {
	appPort        string
	kafkaBrokers   []string
	kafkaTopic     string
	dataServiceURL string
}

type server struct {
	writer    *kafka.Writer
	apiClient *apiclient.Client
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

func main() {
	cfg := loadConfig()

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.kafkaBrokers...),
		Topic:                  cfg.kafkaTopic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	defer writer.Close()

	srv := &server{
		writer:    writer,
		apiClient: apiclient.New(cfg.dataServiceURL, 10*time.Second),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/data", srv.handleData)
	mux.HandleFunc("/search", srv.handleSearch)
	mux.HandleFunc("/reports", srv.handleReports)

	httpServer := &http.Server{
		Addr:         ":" + cfg.appPort,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("api-service started on port %s", cfg.appPort)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("api-service stopped: %v", err)
	}
}

func loadConfig() config {
	brokers := strings.Split(getEnv("KAFKA_BROKERS", "kafka:9092"), ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	return config{
		appPort:        getEnv("APP_PORT", "8080"),
		kafkaBrokers:   brokers,
		kafkaTopic:     getEnv("KAFKA_TOPIC", "social_data"),
		dataServiceURL: getEnv("DATA_SERVICE_URL", "http://data-service:8081"),
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func (s *server) handleData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var msg dataMessage
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&msg); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json"})
		return
	}

	if err := validateDataMessage(msg); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if msg.Type == "comment" {
		exists, err := s.apiClient.CheckPostExists(r.Context(), msg.Comment.PostID)
		if err != nil {
			log.Printf("post existence check failed: %v", err)
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "cannot validate post_id"})
			return
		}
		if !exists {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "post not found"})
			return
		}
	}

	raw, err := json.Marshal(msg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "cannot serialize message"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.writer.WriteMessages(ctx, kafka.Message{
		Key:   buildKafkaMessageKey(msg),
		Value: raw,
	}); err != nil {
		log.Printf("kafka write failed: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "cannot send message to kafka"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func buildKafkaMessageKey(msg dataMessage) []byte {
	switch msg.Type {
	case "comment":
		return []byte("post:" + strconv.FormatInt(msg.Comment.PostID, 10))
	case "post":
		return []byte("author:" + strings.ToLower(strings.TrimSpace(msg.Post.Author)))
	default:
		return []byte("unknown")
	}
}

func validateDataMessage(msg dataMessage) error {
	switch msg.Type {
	case "post":
		if msg.Post == nil {
			return errors.New("field post is required for type=post")
		}
		if strings.TrimSpace(msg.Post.Author) == "" || strings.TrimSpace(msg.Post.Title) == "" || strings.TrimSpace(msg.Post.Body) == "" {
			return errors.New("post.author, post.title and post.body are required")
		}
		if msg.Comment != nil {
			return errors.New("comment must be empty for type=post")
		}
	case "comment":
		if msg.Comment == nil {
			return errors.New("field comment is required for type=comment")
		}
		if msg.Comment.PostID <= 0 || strings.TrimSpace(msg.Comment.Author) == "" || strings.TrimSpace(msg.Comment.Text) == "" {
			return errors.New("comment.post_id, comment.author and comment.text are required")
		}
		if msg.Post != nil {
			return errors.New("post must be empty for type=comment")
		}
	default:
		return errors.New("type must be post or comment")
	}

	return nil
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	s.proxyGet(w, r, "/search")
}

func (s *server) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	s.proxyGet(w, r, "/reports")
}

func (s *server) proxyGet(w http.ResponseWriter, r *http.Request, path string) {
	resp, err := s.apiClient.ProxyGet(r.Context(), path, r.URL.RawQuery)
	if err != nil {
		log.Printf("data-service request failed: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "data-service is unavailable"})
		return
	}
	defer resp.Body.Close()

	if err := apiclient.CopyProxyResponse(w, resp); err != nil {
		log.Printf("cannot copy data-service response: %v", err)
	}
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
