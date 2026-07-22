package di

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/zmskv/feed-service/internal/application/comment"
	"github.com/zmskv/feed-service/internal/application/post"
	"github.com/zmskv/feed-service/internal/config"
	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	"github.com/zmskv/feed-service/internal/infrastructure/repository/memory"
	"github.com/zmskv/feed-service/internal/infrastructure/repository/postgres"
)

type Container struct {
	PostService    *post.Service
	CommentService *comment.Service
	db             *sqlx.DB
}

func (c *Container) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func Build(cfg config.Config) (*Container, error) {
	var postRepo post.Repository
	var commentRepo comment.Repository
	var db *sqlx.DB

	switch cfg.Storage {
	case "memory":
		postRepo = memory.NewPost()
		commentRepo = memory.NewComment()
	case "postgres":
		var err error
		db, err = sqlx.Connect("pgx", cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("di: connect postgres: %w", err)
		}
		db.SetMaxOpenConns(25)
		postRepo = postgres.NewPost(db)
		commentRepo = postgres.NewComment(db)
	default:
		return nil, fmt.Errorf("di: unknown storage %q", cfg.Storage)
	}

	pub := noopPublisher{}

	return &Container{
		PostService:    post.NewService(postRepo),
		CommentService: comment.NewService(commentRepo, postRepo, pub),
		db:             db,
	}, nil
}

type noopPublisher struct{}

func (noopPublisher) Publish(*domainComment.Comment) {}
