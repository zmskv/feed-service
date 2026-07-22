package comment

import "errors"

var (
	ErrEmptyBody   = errors.New("comment: body must not be empty")
	ErrBodyTooLong = errors.New("comment: body exceeds 2000 characters")
	ErrNotFound    = errors.New("comment: not found")
)
