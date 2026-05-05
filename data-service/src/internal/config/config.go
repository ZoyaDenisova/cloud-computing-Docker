package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	AppPort      string
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	KafkaBrokers []string
	KafkaTopic   string
}

func Load() (Config, error) {
	cfg := Config{
		AppPort:      getEnv("APP_PORT", "8081"),
		DBHost:       getEnv("DB_HOST", "db"),
		DBPort:       getEnv("DB_PORT", "5432"),
		DBUser:       strings.TrimSpace(os.Getenv("DB_USER")),
		DBPassword:   strings.TrimSpace(os.Getenv("DB_PASSWORD")),
		DBName:       strings.TrimSpace(os.Getenv("DB_NAME")),
		KafkaBrokers: splitCSV(getEnv("KAFKA_BROKERS", "kafka:9092")),
		KafkaTopic:   getEnv("KAFKA_TOPIC", "social_data"),
	}

	if cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBName == "" {
		return Config{}, errors.New("DB_USER, DB_PASSWORD and DB_NAME must be set")
	}

	return cfg, nil
}

func (c Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost,
		c.DBPort,
		c.DBUser,
		c.DBPassword,
		c.DBName,
	)
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
