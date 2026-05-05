package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"api-service/internal/apiclient"
	"api-service/internal/config"
	"api-service/internal/httpapi"
	"api-service/internal/messaging"
	"github.com/segmentio/kafka-go"
)

func main() {
	cfg := config.Load()

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.KafkaBrokers...),
		Topic:                  cfg.KafkaTopic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	defer writer.Close()

	apiServer := httpapi.New(
		messaging.NewKafkaProducer(writer),
		apiclient.New(cfg.DataServiceURL, 10*time.Second),
	)

	httpServer := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      apiServer.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("api-service started on port %s", cfg.AppPort)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("api-service stopped: %v", err)
	}
}
