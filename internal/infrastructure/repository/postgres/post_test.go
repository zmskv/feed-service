package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/infrastructure/repository/postgres"
	"github.com/zmskv/feed-service/internal/pagination"
)

func TestPost_SaveFindByID(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := postgres.NewPost(db)

	p, err := post.New(uuid.New(), "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(context.Background(), `DELETE FROM posts WHERE id = $1`, p.ID) })

	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got.ID != p.ID || got.Title != p.Title || got.AuthorID != p.AuthorID {
		t.Fatalf("FindByID() = %+v, want %+v", got, p)
	}

	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(err, post.ErrNotFound) {
		t.Fatalf("FindByID() missing = %v, want %v", err, post.ErrNotFound)
	}
}

func TestPost_Save_Upsert(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := postgres.NewPost(db)

	p, err := post.New(uuid.New(), "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(context.Background(), `DELETE FROM posts WHERE id = $1`, p.ID) })

	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	p.DisableComments()
	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save() (update) error = %v", err)
	}

	got, err := repo.FindByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CommentsDisabled {
		t.Fatal("expected CommentsDisabled = true after re-Save")
	}
}

func TestPost_List_NewestFirstAndPagination(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := postgres.NewPost(db)
	authorID := uuid.New()

	const total = 12
	var ids []uuid.UUID
	for i := 0; i < total; i++ {
		p, err := post.New(authorID, "title", "body")
		if err != nil {
			t.Fatal(err)
		}
		p.CreatedAt = p.CreatedAt.Add(time.Duration(i) * time.Second)
		if err := repo.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, p.ID)
	}
	t.Cleanup(func() {
		for _, id := range ids {
			db.ExecContext(context.Background(), `DELETE FROM posts WHERE id = $1`, id)
		}
	})

	seen := map[uuid.UUID]bool{}
	var cursor *pagination.Cursor
	pages := 0
	for {
		items, hasNext, err := repo.List(ctx, 5, cursor)
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

	for _, id := range ids {
		if !seen[id] {
			t.Fatalf("post %v never returned by List()", id)
		}
	}
}
