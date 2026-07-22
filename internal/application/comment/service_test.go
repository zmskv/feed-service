package comment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/zmskv/feed-service/internal/application/comment"
	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	domainPost "github.com/zmskv/feed-service/internal/domain/post"
)

func newPost(t *testing.T, disabled bool) *domainPost.Post {
	t.Helper()
	p, err := domainPost.New(uuid.New(), "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	if disabled {
		p.DisableComments()
	}
	return p
}

func newService(ctrl *gomock.Controller) (*comment.Service, *MockRepository, *MockPostRepository, *MockPublisher) {
	repo := NewMockRepository(ctrl)
	postRepo := NewMockPostRepository(ctrl)
	pub := NewMockPublisher(ctrl)
	return comment.NewService(repo, postRepo, pub), repo, postRepo, pub
}

func TestService_Create_PostNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, _, postRepo, _ := newService(ctrl)

	postID := uuid.New()
	postRepo.EXPECT().FindByID(gomock.Any(), postID).Return(nil, domainPost.ErrNotFound)

	if _, err := svc.Create(context.Background(), postID, nil, uuid.New(), "hi"); !errors.Is(err, domainPost.ErrNotFound) {
		t.Fatalf("err = %v, want %v", err, domainPost.ErrNotFound)
	}
}

func TestService_Create_CommentsDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, _, postRepo, _ := newService(ctrl)

	p := newPost(t, true)
	postRepo.EXPECT().FindByID(gomock.Any(), p.ID).Return(p, nil)

	if _, err := svc.Create(context.Background(), p.ID, nil, uuid.New(), "hi"); !errors.Is(err, domainPost.ErrCommentsDisabled) {
		t.Fatalf("err = %v, want %v", err, domainPost.ErrCommentsDisabled)
	}
}

func TestService_Create_ParentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, repo, postRepo, _ := newService(ctrl)

	p := newPost(t, false)
	missingParent := uuid.New()
	postRepo.EXPECT().FindByID(gomock.Any(), p.ID).Return(p, nil)
	repo.EXPECT().FindByID(gomock.Any(), missingParent).Return(nil, domainComment.ErrNotFound)

	if _, err := svc.Create(context.Background(), p.ID, &missingParent, uuid.New(), "hi"); !errors.Is(err, domainComment.ErrNotFound) {
		t.Fatalf("err = %v, want %v", err, domainComment.ErrNotFound)
	}
}

func TestService_Create_ParentBelongsToDifferentPost(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, repo, postRepo, _ := newService(ctrl)

	p := newPost(t, false)
	otherPost := newPost(t, false)
	parent, err := domainComment.New(otherPost.ID, nil, uuid.New(), "on the other post")
	if err != nil {
		t.Fatal(err)
	}

	postRepo.EXPECT().FindByID(gomock.Any(), p.ID).Return(p, nil)
	repo.EXPECT().FindByID(gomock.Any(), parent.ID).Return(parent, nil)

	_, err = svc.Create(context.Background(), p.ID, &parent.ID, uuid.New(), "hi")
	if !errors.Is(err, comment.ErrParentMismatch) {
		t.Fatalf("err = %v, want %v", err, comment.ErrParentMismatch)
	}
}

func TestService_Create_BodyTooLong(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, _, postRepo, _ := newService(ctrl)

	p := newPost(t, false)
	postRepo.EXPECT().FindByID(gomock.Any(), p.ID).Return(p, nil)

	body := make([]byte, domainComment.MaxBodyLength+1)
	for i := range body {
		body[i] = 'a'
	}

	if _, err := svc.Create(context.Background(), p.ID, nil, uuid.New(), string(body)); !errors.Is(err, domainComment.ErrBodyTooLong) {
		t.Fatalf("err = %v, want %v", err, domainComment.ErrBodyTooLong)
	}
}

func TestService_Create_PublishesToSubscribers(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, repo, postRepo, pub := newService(ctrl)

	p := newPost(t, false)
	postRepo.EXPECT().FindByID(gomock.Any(), p.ID).Return(p, nil)
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	pub.EXPECT().Publish(gomock.Any())

	c, err := svc.Create(context.Background(), p.ID, nil, uuid.New(), "hello")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if c.Body != "hello" {
		t.Fatalf("Body = %q, want %q", c.Body, "hello")
	}
}

func TestService_ListByPost(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, repo, _, _ := newService(ctrl)
	ctx := context.Background()

	p := newPost(t, false)
	top, err := domainComment.New(p.ID, nil, uuid.New(), "top-level")
	if err != nil {
		t.Fatal(err)
	}

	repo.EXPECT().ListByParent(gomock.Any(), p.ID, nil, 10, nil).Return([]*domainComment.Comment{top}, false, nil)

	items, _, err := svc.ListByPost(ctx, p.ID, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != top.ID {
		t.Fatalf("ListByPost() = %+v, want only the top-level comment", items)
	}

	repo.EXPECT().ListByParent(gomock.Any(), p.ID, &top.ID, 10, nil).Return(nil, false, nil)
	replies, _, err := svc.ListReplies(ctx, top, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 0 {
		t.Fatalf("ListReplies() = %+v, want none", replies)
	}
}
