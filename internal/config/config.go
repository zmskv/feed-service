package config

import (
	"flag"
	"fmt"
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
	dsn := flag.String("dsn", envOr("DSN", ""), "full postgres connection string (overrides the PG* vars below if set)")

	pgHost := flag.String("pg-host", envOr("PGHOST", "localhost"), "postgres host")
	pgPort := flag.String("pg-port", envOr("PGPORT", "5432"), "postgres port")
	pgUser := flag.String("pg-user", envOr("PGUSER", "feed"), "postgres user")
	pgPassword := flag.String("pg-password", envOr("PGPASSWORD", "feed"), "postgres password")
	pgDatabase := flag.String("pg-database", envOr("PGDATABASE", "feed"), "postgres database name")
	pgSSLMode := flag.String("pg-sslmode", envOr("PGSSLMODE", "disable"), "postgres sslmode")
	flag.Parse()

	resolvedDSN := *dsn
	if resolvedDSN == "" {
		resolvedDSN = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			*pgUser, *pgPassword, *pgHost, *pgPort, *pgDatabase, *pgSSLMode)
	}

	return Config{
		Addr:    *addr,
		Storage: *storage,
		DSN:     resolvedDSN,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
