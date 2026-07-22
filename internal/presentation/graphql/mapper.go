package graphql

import (
	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	domainPost "github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/presentation/graphql/generated"
)

func postToModel(p *domainPost.Post) *generated.Post {
	return &generated.Post{
		ID:               p.ID.String(),
		AuthorID:         p.AuthorID.String(),
		Title:            p.Title,
		Body:             p.Body,
		CommentsDisabled: p.CommentsDisabled,
		CreatedAt:        p.CreatedAt,
	}
}

func commentToModel(c *domainComment.Comment) *generated.Comment {
	return &generated.Comment{
		ID:        c.ID.String(),
		PostID:    c.PostID.String(),
		AuthorID:  c.AuthorID.String(),
		Body:      c.Body,
		CreatedAt: c.CreatedAt,
	}
}
