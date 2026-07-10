package config

import (
	"log"
	"os"
	"time"
)

type Config struct {
	ServerPort         string
	DatabaseURL        string
	JWKSUrl            string
	AllowedOrigins     string
	KafkaBrokers       string
	OutboxPollInterval time.Duration
}

func Load() *Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	return &Config{
		ServerPort:         getEnv("SERVER_PORT", "8080"),
		DatabaseURL:        dbURL,
		JWKSUrl:            getEnv("JWKS_URL", "http://localhost:8180/realms/appraisal/protocol/openid-connect/certs"),
		AllowedOrigins:     getEnv("ALLOWED_ORIGINS", "*"),
		KafkaBrokers:       getEnv("KAFKA_BROKERS", "localhost:9092"),
		OutboxPollInterval: getDurationEnv("OUTBOX_POLL_INTERVAL", time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Fatalf("invalid %s: %v", key, err)
		}
		return d
	}
	return fallback
}
