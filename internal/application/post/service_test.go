package post_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	apppost "github.com/zmskv/feed-service/internal/application/post"
	domainpost "github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
)

type fakeRepo struct {
	posts map[uuid.UUID]*domainpost.Post
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{posts: make(map[uuid.UUID]*domainpost.Post)}
}

func (r *fakeRepo) Save(_ context.Context, p *domainpost.Post) error {
	r.posts[p.ID] = p
	return nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*domainpost.Post, error) {
	p, ok := r.posts[id]
	if !ok {
		return nil, domainpost.ErrNotFound
	}
	return p, nil
}

func (r *fakeRepo) List(_ context.Context, first int, _ *pagination.Cursor) ([]*domainpost.Post, bool, error) {
	var out []*domainpost.Post
	for _, p := range r.posts {
		out = append(out, p)
	}
	if len(out) > first {
		return out[:first], true, nil
	}
	return out, false, nil
}

func TestService_Create(t *testing.T) {
	svc := apppost.NewService(newFakeRepo())

	p, err := svc.Create(context.Background(), uuid.New(), "title", "body")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if p.Title != "title" {
		t.Fatalf("Title = %q, want %q", p.Title, "title")
	}

	if _, err := svc.Create(context.Background(), uuid.New(), "", "body"); !errors.Is(err, domainpost.ErrEmptyTitle) {
		t.Fatalf("Create() with empty title: err = %v, want %v", err, domainpost.ErrEmptyTitle)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	svc := apppost.NewService(newFakeRepo())

	if _, err := svc.Get(context.Background(), uuid.New()); !errors.Is(err, domainpost.ErrNotFound) {
		t.Fatalf("Get() err = %v, want %v", err, domainpost.ErrNotFound)
	}
}

func TestService_DisableComments(t *testing.T) {
	repo := newFakeRepo()
	svc := apppost.NewService(repo)
	authorID := uuid.New()

	p, err := svc.Create(context.Background(), authorID, "title", "body")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("forbidden for non-author", func(t *testing.T) {
		if err := svc.DisableComments(context.Background(), p.ID, uuid.New()); !errors.Is(err, apppost.ErrForbidden) {
			t.Fatalf("DisableComments() err = %v, want %v", err, apppost.ErrForbidden)
		}
	})

	t.Run("succeeds for author", func(t *testing.T) {
		if err := svc.DisableComments(context.Background(), p.ID, authorID); err != nil {
			t.Fatalf("DisableComments() error = %v", err)
		}
		got, err := svc.Get(context.Background(), p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !got.CommentsDisabled {
			t.Fatal("expected CommentsDisabled = true")
		}
	})
}
