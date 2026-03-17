package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"app/internal/config"
	"app/internal/httpapi"
	"app/internal/storage"
	"app/internal/todo"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	db, err := storage.ConnectWithRetry(context.Background(), cfg.DSN(), 20, 2*time.Second)
	if err != nil {
		log.Fatalf("cannot connect to db: %v", err)
	}
	defer db.Close()

	repo := todo.NewPostgresRepository(db)
	service := todo.NewService(repo)
	handler := httpapi.NewHandler(service)

	mux := http.NewServeMux()
	handler.Register(mux)

	server := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      httpapi.LoggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("server started on port %s", cfg.AppPort)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped: %v", err)
	}
}
