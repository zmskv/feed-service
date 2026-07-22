package config

import (
	"flag"
	"os"
)

type Config struct {
	Addr    string
	Storage string // "memory" | "postgres"
	DSN     string
}

func Load() Config {
	addr := flag.String("addr", envOr("ADDR", ":8080"), "http listen address")
	storage := flag.String("storage", envOr("STORAGE", "memory"), "storage backend: memory | postgres")
	dsn := flag.String("dsn", envOr("DSN", ""), "postgres connection string")
	flag.Parse()

	return Config{
		Addr:    *addr,
		Storage: *storage,
		DSN:     *dsn,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
