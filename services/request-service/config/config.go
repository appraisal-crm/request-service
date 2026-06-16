package config

import "os"

type Config struct {
	ServerPort     string
	DatabaseURL    string
	JWKSUrl        string
	AllowedOrigins string
}

func Load() *Config {
	return &Config{
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://appraisal:appraisal@localhost:5433/request_db"),
		JWKSUrl:        getEnv("JWKS_URL", "http://localhost:8180/realms/appraisal/protocol/openid-connect/certs"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "*"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
