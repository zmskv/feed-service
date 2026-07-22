package post

import (
	"time"

	"github.com/google/uuid"
)

type Post struct {
	ID               uuid.UUID
	AuthorID         uuid.UUID
	Title            string
	Body             string
	CommentsDisabled bool
	CreatedAt        time.Time
}

func New(authorID uuid.UUID, title, body string) (*Post, error) {
	if title == "" {
		return nil, ErrEmptyTitle
	}
	if body == "" {
		return nil, ErrEmptyBody
	}
	return &Post{
		ID:        uuid.New(),
		AuthorID:  authorID,
		Title:     title,
		Body:      body,
		CreatedAt: time.Now(),
	}, nil
}

func (p *Post) DisableComments() {
	p.CommentsDisabled = true
}

func (p *Post) CanAcceptComments() error {
	if p.CommentsDisabled {
		return ErrCommentsDisabled
	}
	return nil
}
