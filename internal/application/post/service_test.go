package post_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/zmskv/feed-service/internal/application/post"
	domainPost "github.com/zmskv/feed-service/internal/domain/post"
)

func TestService_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockRepository(ctrl)
	svc := post.NewService(repo)

	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	p, err := svc.Create(context.Background(), uuid.New(), "title", "body")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if p.Title != "title" {
		t.Fatalf("Title = %q, want %q", p.Title, "title")
	}

	if _, err := svc.Create(context.Background(), uuid.New(), "", "body"); !errors.Is(err, domainPost.ErrEmptyTitle) {
		t.Fatalf("Create() with empty title: err = %v, want %v", err, domainPost.ErrEmptyTitle)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockRepository(ctrl)
	svc := post.NewService(repo)

	id := uuid.New()
	repo.EXPECT().FindByID(gomock.Any(), id).Return(nil, domainPost.ErrNotFound)

	if _, err := svc.Get(context.Background(), id); !errors.Is(err, domainPost.ErrNotFound) {
		t.Fatalf("Get() err = %v, want %v", err, domainPost.ErrNotFound)
	}
}

func TestService_DisableComments_ForbiddenForNonAuthor(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockRepository(ctrl)
	svc := post.NewService(repo)

	authorID := uuid.New()
	p, err := domainPost.New(authorID, "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	repo.EXPECT().FindByID(gomock.Any(), p.ID).Return(p, nil)

	if err := svc.DisableComments(context.Background(), p.ID, uuid.New()); !errors.Is(err, post.ErrForbidden) {
		t.Fatalf("DisableComments() err = %v, want %v", err, post.ErrForbidden)
	}
}

func TestService_DisableComments_SucceedsForAuthor(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockRepository(ctrl)
	svc := post.NewService(repo)

	authorID := uuid.New()
	p, err := domainPost.New(authorID, "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	repo.EXPECT().FindByID(gomock.Any(), p.ID).Return(p, nil)
	repo.EXPECT().Save(gomock.Any(), p).Return(nil)

	if err := svc.DisableComments(context.Background(), p.ID, authorID); err != nil {
		t.Fatalf("DisableComments() error = %v", err)
	}
	if !p.CommentsDisabled {
		t.Fatal("expected CommentsDisabled = true")
	}
}
