package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"api-service/internal/model"
)

type MessageProducer interface {
	Produce(ctx context.Context, key, value []byte) error
}

type DataServiceClient interface {
	CheckPostExists(parentCtx context.Context, postID int64) (bool, error)
	ProxyGet(parentCtx context.Context, path, rawQuery string) (*http.Response, error)
}

type Server struct {
	writer    MessageProducer
	apiClient DataServiceClient
}

func New(writer MessageProducer, apiClient DataServiceClient) *Server {
	return &Server{
		writer:    writer,
		apiClient: apiClient,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/data", s.handleData)
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/reports", s.handleReports)
	return LoggingMiddleware(mux)
}

func (s *Server) handleData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var msg model.DataMessage
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&msg); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid json"})
		return
	}

	if err := model.ValidateDataMessage(msg); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	if msg.Type == "comment" {
		exists, err := s.apiClient.CheckPostExists(r.Context(), msg.Comment.PostID)
		if err != nil {
			log.Printf("post existence check failed: %v", err)
			writeJSON(w, http.StatusBadGateway, model.ErrorResponse{Error: "cannot validate post_id"})
			return
		}
		if !exists {
			writeJSON(w, http.StatusNotFound, model.ErrorResponse{Error: "post not found"})
			return
		}
	}

	raw, err := json.Marshal(msg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "cannot serialize message"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.writer.Produce(ctx, model.BuildKafkaMessageKey(msg), raw); err != nil {
		log.Printf("kafka write failed: %v", err)
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{Error: "cannot send message to kafka"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	s.proxyGet(w, r, "/search")
}

func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	s.proxyGet(w, r, "/reports")
}

func (s *Server) proxyGet(w http.ResponseWriter, r *http.Request, path string) {
	resp, err := s.apiClient.ProxyGet(r.Context(), path, r.URL.RawQuery)
	if err != nil {
		log.Printf("data-service request failed: %v", err)
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{Error: "data-service is unavailable"})
		return
	}
	defer resp.Body.Close()

	if err := copyProxyResponse(w, resp); err != nil {
		log.Printf("cannot copy data-service response: %v", err)
	}
}

func copyProxyResponse(w http.ResponseWriter, resp *http.Response) error {
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)

	_, err := io.Copy(w, resp.Body)
	return err
}
