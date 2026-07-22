package comment

import (
	"context"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/domain/comment"
	"github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
)

type PostRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*post.Post, error)
}

type CommentRepository interface {
	Save(ctx context.Context, c *comment.Comment) error
	FindByID(ctx context.Context, id uuid.UUID) (*comment.Comment, error)
	ListByParent(ctx context.Context, postID uuid.UUID, parentID *uuid.UUID, first int, after *pagination.Cursor) ([]*comment.Comment, bool, error)
}

type Publisher interface {
	Publish(c *comment.Comment)
}

type Service struct {
	repo     CommentRepository
	postRepo PostRepository
	pub      Publisher
}

func NewService(repo CommentRepository, postRepo PostRepository, pub Publisher) *Service {
	return &Service{repo: repo, postRepo: postRepo, pub: pub}
}

func (s *Service) Create(ctx context.Context, postID uuid.UUID, parentID *uuid.UUID, authorID uuid.UUID, body string) (*comment.Comment, error) {
	p, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if err := p.CanAcceptComments(); err != nil {
		return nil, err
	}

	if parentID != nil {
		parent, err := s.repo.FindByID(ctx, *parentID)
		if err != nil {
			return nil, err
		}
		if parent.PostID != postID {
			return nil, ErrParentMismatch
		}
	}

	c, err := comment.New(postID, parentID, authorID, body)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, c); err != nil {
		return nil, err
	}

	if s.pub != nil {
		s.pub.Publish(c)
	}
	return c, nil
}

func (s *Service) ListByPost(ctx context.Context, postID uuid.UUID, first int, after *pagination.Cursor) ([]*comment.Comment, bool, error) {
	return s.repo.ListByParent(ctx, postID, nil, pagination.NormalizeFirst(first), after)
}

func (s *Service) ListReplies(ctx context.Context, parent *comment.Comment, first int, after *pagination.Cursor) ([]*comment.Comment, bool, error) {
	return s.repo.ListByParent(ctx, parent.PostID, &parent.ID, pagination.NormalizeFirst(first), after)
}
