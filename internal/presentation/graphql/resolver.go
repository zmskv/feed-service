package graphql

import (
	"context"

	"github.com/google/uuid"

	appComment "github.com/zmskv/feed-service/internal/application/comment"
	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	domainPost "github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
)

type PostService interface {
	Get(ctx context.Context, id uuid.UUID) (*domainPost.Post, error)
	List(ctx context.Context, first int, after *pagination.Cursor) ([]*domainPost.Post, bool, error)
	Create(ctx context.Context, authorID uuid.UUID, title, body string) (*domainPost.Post, error)
	DisableComments(ctx context.Context, id, requesterID uuid.UUID) error
}

type CommentService interface {
	ListByPostsBatch(ctx context.Context, postIDs []uuid.UUID, first int, after *pagination.Cursor) (map[uuid.UUID]*appComment.Page, error)
	ListRepliesByParentsBatch(ctx context.Context, parentIDs []uuid.UUID, first int, after *pagination.Cursor) (map[uuid.UUID]*appComment.Page, error)
	Create(ctx context.Context, postID uuid.UUID, parentID *uuid.UUID, authorID uuid.UUID, body string) (*domainComment.Comment, error)
}

type Subscriber interface {
	Subscribe(postID uuid.UUID) (<-chan *domainComment.Comment, func())
}

type Resolver struct {
	posts    PostService
	comments CommentService
	sub      Subscriber
}

func NewResolver(posts PostService, comments CommentService, sub Subscriber) *Resolver {
	return &Resolver{posts: posts, comments: comments, sub: sub}
}
