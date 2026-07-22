package di

import (
	"fmt"

	"github.com/zmskv/feed-service/internal/application/comment"
	"github.com/zmskv/feed-service/internal/application/post"
	"github.com/zmskv/feed-service/internal/config"
	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	"github.com/zmskv/feed-service/internal/infrastructure/repository/memory"
)

type Container struct {
	PostService    *post.Service
	CommentService *comment.Service
}

func Build(cfg config.Config) (*Container, error) {
	var postRepo post.Repository
	var commentRepo comment.Repository

	switch cfg.Storage {
	case "memory":
		postRepo = memory.NewPost()
		commentRepo = memory.NewComment()
	case "postgres":
		return nil, fmt.Errorf("di: postgres storage not implemented yet")
	default:
		return nil, fmt.Errorf("di: unknown storage %q", cfg.Storage)
	}

	pub := noopPublisher{}

	return &Container{
		PostService:    post.NewService(postRepo),
		CommentService: comment.NewService(commentRepo, postRepo, pub),
	}, nil
}


type noopPublisher struct{}

func (noopPublisher) Publish(*domainComment.Comment) {}
