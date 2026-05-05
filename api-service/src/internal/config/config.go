package config

import (
	"os"
	"strings"
)

type Config struct {
	AppPort        string
	KafkaBrokers   []string
	KafkaTopic     string
	DataServiceURL string
}

func Load() Config {
	brokers := strings.Split(getEnv("KAFKA_BROKERS", "kafka:9092"), ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	return Config{
		AppPort:        getEnv("APP_PORT", "8080"),
		KafkaBrokers:   brokers,
		KafkaTopic:     getEnv("KAFKA_TOPIC", "social_data"),
		DataServiceURL: getEnv("DATA_SERVICE_URL", "http://data-service:8081"),
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
