package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"data-service/internal/config"
	"data-service/internal/consumer"
	"data-service/internal/httpapi"
	"data-service/internal/messaging"
	"data-service/internal/storage"
	"github.com/segmentio/kafka-go"
)

const (
	defaultRetryAttempts = 30
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	db, err := storage.ConnectWithRetry(context.Background(), cfg.DSN(), defaultRetryAttempts, 2*time.Second)
	if err != nil {
		log.Fatalf("cannot connect to db: %v", err)
	}
	defer db.Close()

	st := storage.New(db)

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.KafkaBrokers,
		Topic:       cfg.KafkaTopic,
		Partition:   0,
		StartOffset: kafka.FirstOffset,
		MinBytes:    1,
		MaxBytes:    10e6,
	})
	defer reader.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go consumer.Run(ctx, messaging.NewKafkaReader(reader), st)

	apiServer := httpapi.New(st)
	httpServer := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      apiServer.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("data-service started on port %s", cfg.AppPort)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("data-service stopped: %v", err)
	}
}
