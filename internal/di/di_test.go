package di_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/config"
	"github.com/zmskv/feed-service/internal/di"
)

func TestBuild_Memory_EndToEnd(t *testing.T) {
	ctx := context.Background()

	c, err := di.Build(config.Config{Storage: "memory"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	authorID := uuid.New()
	p, err := c.PostService.Create(ctx, authorID, "title", "body")
	if err != nil {
		t.Fatalf("PostService.Create() error = %v", err)
	}

	cm, err := c.CommentService.Create(ctx, p.ID, nil, uuid.New(), "first comment")
	if err != nil {
		t.Fatalf("CommentService.Create() error = %v", err)
	}

	items, _, err := c.CommentService.ListByPost(ctx, p.ID, 10, nil)
	if err != nil {
		t.Fatalf("ListByPost() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != cm.ID {
		t.Fatalf("ListByPost() = %+v, want only %v", items, cm.ID)
	}

	if err := c.PostService.DisableComments(ctx, p.ID, authorID); err != nil {
		t.Fatalf("DisableComments() error = %v", err)
	}
	if _, err := c.CommentService.Create(ctx, p.ID, nil, uuid.New(), "should fail"); err == nil {
		t.Fatal("Create() after DisableComments() = nil error, want ErrCommentsDisabled")
	}
}

func TestBuild_UnknownStorage(t *testing.T) {
	if _, err := di.Build(config.Config{Storage: "carrier-pigeon"}); err == nil {
		t.Fatal("Build() with unknown storage = nil error, want error")
	}
}

func TestBuild_Postgres_InvalidDSN(t *testing.T) {
	_, err := di.Build(config.Config{Storage: "postgres", DSN: "postgres://user:pass@127.0.0.1:59999/nope?sslmode=disable"})
	if err == nil {
		t.Fatal("Build() with unreachable postgres = nil error, want a connection error")
	}
}

func TestBuild_Postgres_EndToEnd(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping postgres integration test")
	}
	ctx := context.Background()

	c, err := di.Build(config.Config{Storage: "postgres", DSN: dsn})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	defer c.Close()

	authorID := uuid.New()
	p, err := c.PostService.Create(ctx, authorID, "title", "body")
	if err != nil {
		t.Fatalf("PostService.Create() error = %v", err)
	}

	cm, err := c.CommentService.Create(ctx, p.ID, nil, uuid.New(), "first comment")
	if err != nil {
		t.Fatalf("CommentService.Create() error = %v", err)
	}

	items, _, err := c.CommentService.ListByPost(ctx, p.ID, 10, nil)
	if err != nil {
		t.Fatalf("ListByPost() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != cm.ID {
		t.Fatalf("ListByPost() = %+v, want only %v", items, cm.ID)
	}
}
