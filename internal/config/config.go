package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr    string
	Storage string // "memory" | "postgres"
	DSN     string
}

func Load() Config {
	_ = godotenv.Load()

	dsn := envOr("DSN", "")
	if dsn == "" {
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			envOr("PGUSER", "feed"), envOr("PGPASSWORD", "feed"),
			envOr("PGHOST", "localhost"), envOr("PGPORT", "5432"),
			envOr("PGDATABASE", "feed"), envOr("PGSSLMODE", "disable"))
	}

	return Config{
		Addr:    envOr("ADDR", ":8080"),
		Storage: envOr("STORAGE", "memory"),
		DSN:     dsn,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
