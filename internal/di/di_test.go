package di_test

import (
	"context"
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

func TestBuild_PostgresNotYetImplemented(t *testing.T) {
	if _, err := di.Build(config.Config{Storage: "postgres"}); err == nil {
		t.Fatal("Build() with postgres storage = nil error, want error (not implemented until M5)")
	}
}
