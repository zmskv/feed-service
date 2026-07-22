package comment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/application/comment"
	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	domainPost "github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
)

type fakePostRepo struct {
	posts map[uuid.UUID]*domainPost.Post
}

func (r *fakePostRepo) FindByID(_ context.Context, id uuid.UUID) (*domainPost.Post, error) {
	p, ok := r.posts[id]
	if !ok {
		return nil, domainPost.ErrNotFound
	}
	return p, nil
}

type fakeCommentRepo struct {
	comments map[uuid.UUID]*domainComment.Comment
}

func newFakeCommentRepo() *fakeCommentRepo {
	return &fakeCommentRepo{comments: make(map[uuid.UUID]*domainComment.Comment)}
}

func (r *fakeCommentRepo) Save(_ context.Context, c *domainComment.Comment) error {
	r.comments[c.ID] = c
	return nil
}

func (r *fakeCommentRepo) FindByID(_ context.Context, id uuid.UUID) (*domainComment.Comment, error) {
	c, ok := r.comments[id]
	if !ok {
		return nil, domainComment.ErrNotFound
	}
	return c, nil
}

func (r *fakeCommentRepo) ListByParent(_ context.Context, postID uuid.UUID, parentID *uuid.UUID, first int, _ *pagination.Cursor) ([]*domainComment.Comment, bool, error) {
	var out []*domainComment.Comment
	for _, c := range r.comments {
		if c.PostID != postID {
			continue
		}
		if (c.ParentID == nil) != (parentID == nil) {
			continue
		}
		if c.ParentID != nil && parentID != nil && *c.ParentID != *parentID {
			continue
		}
		out = append(out, c)
	}
	if len(out) > first {
		return out[:first], true, nil
	}
	return out, false, nil
}

type fakePublisher struct {
	published []*domainComment.Comment
}

func (p *fakePublisher) Publish(c *domainComment.Comment) {
	p.published = append(p.published, c)
}

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

func TestService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("post not found", func(t *testing.T) {
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainPost.Post{}}
		svc := comment.NewService(newFakeCommentRepo(), postRepo, &fakePublisher{})

		_, err := svc.Create(ctx, uuid.New(), nil, uuid.New(), "hi")
		if !errors.Is(err, domainPost.ErrNotFound) {
			t.Fatalf("err = %v, want %v", err, domainPost.ErrNotFound)
		}
	})

	t.Run("comments disabled", func(t *testing.T) {
		p := newPost(t, true)
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainPost.Post{p.ID: p}}
		svc := comment.NewService(newFakeCommentRepo(), postRepo, &fakePublisher{})

		_, err := svc.Create(ctx, p.ID, nil, uuid.New(), "hi")
		if !errors.Is(err, domainPost.ErrCommentsDisabled) {
			t.Fatalf("err = %v, want %v", err, domainPost.ErrCommentsDisabled)
		}
	})

	t.Run("parent not found", func(t *testing.T) {
		p := newPost(t, false)
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainPost.Post{p.ID: p}}
		svc := comment.NewService(newFakeCommentRepo(), postRepo, &fakePublisher{})

		missingParent := uuid.New()
		_, err := svc.Create(ctx, p.ID, &missingParent, uuid.New(), "hi")
		if !errors.Is(err, domainComment.ErrNotFound) {
			t.Fatalf("err = %v, want %v", err, domainComment.ErrNotFound)
		}
	})

	t.Run("parent belongs to a different post", func(t *testing.T) {
		p := newPost(t, false)
		otherPost := newPost(t, false)
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainPost.Post{p.ID: p, otherPost.ID: otherPost}}
		commentRepo := newFakeCommentRepo()

		parent, err := domainComment.New(otherPost.ID, nil, uuid.New(), "on the other post")
		if err != nil {
			t.Fatal(err)
		}
		if err := commentRepo.Save(ctx, parent); err != nil {
			t.Fatal(err)
		}

		svc := comment.NewService(commentRepo, postRepo, &fakePublisher{})
		_, err = svc.Create(ctx, p.ID, &parent.ID, uuid.New(), "hi")
		if !errors.Is(err, comment.ErrParentMismatch) {
			t.Fatalf("err = %v, want %v", err, comment.ErrParentMismatch)
		}
	})

	t.Run("body too long", func(t *testing.T) {
		p := newPost(t, false)
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainPost.Post{p.ID: p}}
		svc := comment.NewService(newFakeCommentRepo(), postRepo, &fakePublisher{})

		body := make([]byte, domainComment.MaxBodyLength+1)
		for i := range body {
			body[i] = 'a'
		}
		_, err := svc.Create(ctx, p.ID, nil, uuid.New(), string(body))
		if !errors.Is(err, domainComment.ErrBodyTooLong) {
			t.Fatalf("err = %v, want %v", err, domainComment.ErrBodyTooLong)
		}
	})

	t.Run("valid comment publishes to subscribers", func(t *testing.T) {
		p := newPost(t, false)
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainPost.Post{p.ID: p}}
		pub := &fakePublisher{}
		svc := comment.NewService(newFakeCommentRepo(), postRepo, pub)

		c, err := svc.Create(ctx, p.ID, nil, uuid.New(), "hello")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if len(pub.published) != 1 || pub.published[0].ID != c.ID {
			t.Fatalf("expected comment %v to be published, got %+v", c.ID, pub.published)
		}
	})
}

func TestService_ListByPost(t *testing.T) {
	ctx := context.Background()
	p := newPost(t, false)
	postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainPost.Post{p.ID: p}}
	commentRepo := newFakeCommentRepo()
	svc := comment.NewService(commentRepo, postRepo, &fakePublisher{})

	top, err := svc.Create(ctx, p.ID, nil, uuid.New(), "top-level")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, p.ID, &top.ID, uuid.New(), "a reply"); err != nil {
		t.Fatal(err)
	}

	items, _, err := svc.ListByPost(ctx, p.ID, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != top.ID {
		t.Fatalf("ListByPost() = %+v, want only the top-level comment", items)
	}

	replies, _, err := svc.ListReplies(ctx, top, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 1 {
		t.Fatalf("ListReplies() = %+v, want 1 reply", replies)
	}
}
