package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/domain/comment"
	"github.com/zmskv/feed-service/internal/infrastructure/repository/memory"
	"github.com/zmskv/feed-service/internal/pagination"
)

func TestComment_SaveFindByID(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewComment()
	postID := uuid.New()

	c, err := comment.New(postID, nil, uuid.New(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, c); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.FindByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got.ID != c.ID {
		t.Fatalf("FindByID() = %+v, want %+v", got, c)
	}

	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(err, comment.ErrNotFound) {
		t.Fatalf("FindByID() missing = %v, want %v", err, comment.ErrNotFound)
	}
}

func TestComment_ListByParent_TopLevelVsReplies(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewComment()
	postID := uuid.New()
	otherPostID := uuid.New()

	top, err := comment.New(postID, nil, uuid.New(), "top")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, top); err != nil {
		t.Fatal(err)
	}

	reply, err := comment.New(postID, &top.ID, uuid.New(), "reply")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, reply); err != nil {
		t.Fatal(err)
	}

	otherPostComment, err := comment.New(otherPostID, nil, uuid.New(), "elsewhere")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, otherPostComment); err != nil {
		t.Fatal(err)
	}

	topLevel, _, err := repo.ListByParent(ctx, postID, nil, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(topLevel) != 1 || topLevel[0].ID != top.ID {
		t.Fatalf("ListByParent(top-level) = %+v, want only %v", topLevel, top.ID)
	}

	replies, _, err := repo.ListByParent(ctx, postID, &top.ID, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 1 || replies[0].ID != reply.ID {
		t.Fatalf("ListByParent(replies) = %+v, want only %v", replies, reply.ID)
	}
}

func TestComment_ListByParent_OldestFirstAndPagination(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewComment()
	postID := uuid.New()

	const total = 15
	var created []*comment.Comment
	for i := 0; i < total; i++ {
		c, err := comment.New(postID, nil, uuid.New(), "hi")
		if err != nil {
			t.Fatal(err)
		}
		c.CreatedAt = c.CreatedAt.Add(time.Duration(i) * time.Second)
		if err := repo.Save(ctx, c); err != nil {
			t.Fatal(err)
		}
		created = append(created, c)
	}

	seen := map[uuid.UUID]bool{}
	var cursor *pagination.Cursor
	var order []uuid.UUID
	for {
		items, hasNext, err := repo.ListByParent(ctx, postID, nil, 5, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range items {
			if seen[c.ID] {
				t.Fatalf("duplicate comment %v across pages", c.ID)
			}
			seen[c.ID] = true
			order = append(order, c.ID)
		}
		if !hasNext {
			break
		}
		last := items[len(items)-1]
		cursor = &pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}

	if len(order) != total {
		t.Fatalf("got %d comments, want %d", len(order), total)
	}
	for i, c := range created { // oldest-first: matches insertion order here
		if order[i] != c.ID {
			t.Fatalf("order[%d] = %v, want %v (oldest-first)", i, order[i], c.ID)
		}
	}
}
