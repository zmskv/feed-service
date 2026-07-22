package comment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	appcomment "github.com/zmskv/feed-service/internal/application/comment"
	domaincomment "github.com/zmskv/feed-service/internal/domain/comment"
	domainpost "github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
)

type fakePostRepo struct {
	posts map[uuid.UUID]*domainpost.Post
}

func (r *fakePostRepo) FindByID(_ context.Context, id uuid.UUID) (*domainpost.Post, error) {
	p, ok := r.posts[id]
	if !ok {
		return nil, domainpost.ErrNotFound
	}
	return p, nil
}

type fakeCommentRepo struct {
	comments map[uuid.UUID]*domaincomment.Comment
}

func newFakeCommentRepo() *fakeCommentRepo {
	return &fakeCommentRepo{comments: make(map[uuid.UUID]*domaincomment.Comment)}
}

func (r *fakeCommentRepo) Save(_ context.Context, c *domaincomment.Comment) error {
	r.comments[c.ID] = c
	return nil
}

func (r *fakeCommentRepo) FindByID(_ context.Context, id uuid.UUID) (*domaincomment.Comment, error) {
	c, ok := r.comments[id]
	if !ok {
		return nil, domaincomment.ErrNotFound
	}
	return c, nil
}

func (r *fakeCommentRepo) ListByParent(_ context.Context, postID uuid.UUID, parentID *uuid.UUID, first int, _ *pagination.Cursor) ([]*domaincomment.Comment, bool, error) {
	var out []*domaincomment.Comment
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
	published []*domaincomment.Comment
}

func (p *fakePublisher) Publish(c *domaincomment.Comment) {
	p.published = append(p.published, c)
}

func newPost(t *testing.T, disabled bool) *domainpost.Post {
	t.Helper()
	p, err := domainpost.New(uuid.New(), "title", "body")
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
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainpost.Post{}}
		svc := appcomment.NewService(newFakeCommentRepo(), postRepo, &fakePublisher{})

		_, err := svc.Create(ctx, uuid.New(), nil, uuid.New(), "hi")
		if !errors.Is(err, domainpost.ErrNotFound) {
			t.Fatalf("err = %v, want %v", err, domainpost.ErrNotFound)
		}
	})

	t.Run("comments disabled", func(t *testing.T) {
		p := newPost(t, true)
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainpost.Post{p.ID: p}}
		svc := appcomment.NewService(newFakeCommentRepo(), postRepo, &fakePublisher{})

		_, err := svc.Create(ctx, p.ID, nil, uuid.New(), "hi")
		if !errors.Is(err, domainpost.ErrCommentsDisabled) {
			t.Fatalf("err = %v, want %v", err, domainpost.ErrCommentsDisabled)
		}
	})

	t.Run("parent not found", func(t *testing.T) {
		p := newPost(t, false)
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainpost.Post{p.ID: p}}
		svc := appcomment.NewService(newFakeCommentRepo(), postRepo, &fakePublisher{})

		missingParent := uuid.New()
		_, err := svc.Create(ctx, p.ID, &missingParent, uuid.New(), "hi")
		if !errors.Is(err, domaincomment.ErrNotFound) {
			t.Fatalf("err = %v, want %v", err, domaincomment.ErrNotFound)
		}
	})

	t.Run("parent belongs to a different post", func(t *testing.T) {
		p := newPost(t, false)
		otherPost := newPost(t, false)
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainpost.Post{p.ID: p, otherPost.ID: otherPost}}
		commentRepo := newFakeCommentRepo()

		parent, err := domaincomment.New(otherPost.ID, nil, uuid.New(), "on the other post")
		if err != nil {
			t.Fatal(err)
		}
		if err := commentRepo.Save(ctx, parent); err != nil {
			t.Fatal(err)
		}

		svc := appcomment.NewService(commentRepo, postRepo, &fakePublisher{})
		_, err = svc.Create(ctx, p.ID, &parent.ID, uuid.New(), "hi")
		if !errors.Is(err, appcomment.ErrParentMismatch) {
			t.Fatalf("err = %v, want %v", err, appcomment.ErrParentMismatch)
		}
	})

	t.Run("body too long", func(t *testing.T) {
		p := newPost(t, false)
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainpost.Post{p.ID: p}}
		svc := appcomment.NewService(newFakeCommentRepo(), postRepo, &fakePublisher{})

		body := make([]byte, domaincomment.MaxBodyLength+1)
		for i := range body {
			body[i] = 'a'
		}
		_, err := svc.Create(ctx, p.ID, nil, uuid.New(), string(body))
		if !errors.Is(err, domaincomment.ErrBodyTooLong) {
			t.Fatalf("err = %v, want %v", err, domaincomment.ErrBodyTooLong)
		}
	})

	t.Run("valid comment publishes to subscribers", func(t *testing.T) {
		p := newPost(t, false)
		postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainpost.Post{p.ID: p}}
		pub := &fakePublisher{}
		svc := appcomment.NewService(newFakeCommentRepo(), postRepo, pub)

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
	postRepo := &fakePostRepo{posts: map[uuid.UUID]*domainpost.Post{p.ID: p}}
	commentRepo := newFakeCommentRepo()
	svc := appcomment.NewService(commentRepo, postRepo, &fakePublisher{})

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
