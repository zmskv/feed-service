package graphql

import (
	"context"

	"github.com/google/uuid"

	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	domainPost "github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
)

type PostService interface {
	Get(ctx context.Context, id uuid.UUID) (*domainPost.Post, error)
	List(ctx context.Context, first int, after *pagination.Cursor) ([]*domainPost.Post, bool, error)
}

type CommentService interface {
	ListByPost(ctx context.Context, postID uuid.UUID, first int, after *pagination.Cursor) ([]*domainComment.Comment, bool, error)
	ListReplies(ctx context.Context, parent *domainComment.Comment, first int, after *pagination.Cursor) ([]*domainComment.Comment, bool, error)
}

type Resolver struct {
	posts    PostService
	comments CommentService
}

func NewResolver(posts PostService, comments CommentService) *Resolver {
	return &Resolver{posts: posts, comments: comments}
}
