package postgres_test

import (
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping postgres integration test")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
