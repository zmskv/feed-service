package memory_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/infrastructure/repository/memory"
	"github.com/zmskv/feed-service/internal/pagination"
)

func TestPost_SaveFindByID(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPost()

	p, err := post.New(uuid.New(), "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got.ID != p.ID {
		t.Fatalf("FindByID() = %+v, want %+v", got, p)
	}

	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(err, post.ErrNotFound) {
		t.Fatalf("FindByID() missing = %v, want %v", err, post.ErrNotFound)
	}
}

func TestPost_List_Pagination(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPost()
	authorID := uuid.New()

	const total = 25
	for i := 0; i < total; i++ {
		p, err := post.New(authorID, "title", "body")
		if err != nil {
			t.Fatal(err)
		}
		p.CreatedAt = p.CreatedAt.Add(time.Duration(i) * time.Second)
		if err := repo.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	seen := map[uuid.UUID]bool{}
	var cursor *pagination.Cursor
	pages := 0
	for {
		items, hasNext, err := repo.List(ctx, 10, cursor)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, p := range items {
			if seen[p.ID] {
				t.Fatalf("duplicate post %v across pages", p.ID)
			}
			seen[p.ID] = true
		}
		if !hasNext {
			break
		}
		last := items[len(items)-1]
		cursor = &pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		if pages > total {
			t.Fatal("too many pages, pagination likely broken")
		}
	}

	if len(seen) != total {
		t.Fatalf("saw %d posts across %d pages, want %d", len(seen), pages, total)
	}
	if pages != 3 { // 10 + 10 + 5
		t.Fatalf("pages = %d, want 3", pages)
	}
}

func TestPost_List_NewestFirst(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPost()
	authorID := uuid.New()

	older, err := post.New(authorID, "older", "body")
	if err != nil {
		t.Fatal(err)
	}
	newer, err := post.New(authorID, "newer", "body")
	if err != nil {
		t.Fatal(err)
	}
	newer.CreatedAt = older.CreatedAt.Add(time.Minute)

	if err := repo.Save(ctx, older); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, newer); err != nil {
		t.Fatal(err)
	}

	items, _, err := repo.List(ctx, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != newer.ID || items[1].ID != older.ID {
		t.Fatalf("List() order = %+v, want newest-first [newer, older]", items)
	}
}

func TestPost_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPost()
	authorID := uuid.New()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			p, err := post.New(authorID, "title", "body")
			if err != nil {
				t.Error(err)
				return
			}
			if err := repo.Save(ctx, p); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	items, _, err := repo.List(ctx, n+1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != n {
		t.Fatalf("List() len = %d, want %d", len(items), n)
	}
}

func TestPost_FindByID_DoesNotAliasInternalStorage(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPost()

	p, err := post.New(uuid.New(), "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.DisableComments()

	again, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.CommentsDisabled {
		t.Fatal("mutating a FindByID() result leaked into the repository's stored copy — FindByID must return an independent copy")
	}
}

func TestPost_Save_DoesNotAliasCallerPointer(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewPost()

	p, err := post.New(uuid.New(), "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	p.DisableComments()

	got, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CommentsDisabled {
		t.Fatal("mutating the pointer passed to Save() after the call leaked into stored state")
	}
}
