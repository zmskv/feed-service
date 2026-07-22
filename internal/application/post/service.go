package post

import (
	"context"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
)

type PostRepository interface {
	Save(ctx context.Context, p *post.Post) error
	FindByID(ctx context.Context, id uuid.UUID) (*post.Post, error)
	List(ctx context.Context, first int, after *pagination.Cursor) ([]*post.Post, bool, error)
}

type Service struct {
	repo PostRepository
}

func NewService(repo PostRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, authorID uuid.UUID, title, body string) (*post.Post, error) {
	p, err := post.New(authorID, title, body)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*post.Post, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, first int, after *pagination.Cursor) ([]*post.Post, bool, error) {
	return s.repo.List(ctx, pagination.NormalizeFirst(first), after)
}

func (s *Service) DisableComments(ctx context.Context, id, requesterID uuid.UUID) error {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if p.AuthorID != requesterID {
		return ErrForbidden
	}
	p.DisableComments()
	return s.repo.Save(ctx, p)
}
