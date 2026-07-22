package comment

import (
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxBodyLength = 2000

type Comment struct {
	ID        uuid.UUID
	PostID    uuid.UUID
	ParentID  *uuid.UUID // nil for top-level comments
	AuthorID  uuid.UUID
	Body      string
	CreatedAt time.Time
}

func New(postID uuid.UUID, parentID *uuid.UUID, authorID uuid.UUID, body string) (*Comment, error) {
	if body == "" {
		return nil, ErrEmptyBody
	}
	if utf8.RuneCountInString(body) > MaxBodyLength {
		return nil, ErrBodyTooLong
	}
	return &Comment{
		ID:        uuid.New(),
		PostID:    postID,
		ParentID:  parentID,
		AuthorID:  authorID,
		Body:      body,
		CreatedAt: time.Now(),
	}, nil
}
