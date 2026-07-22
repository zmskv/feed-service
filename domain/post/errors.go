package post

import "errors"

var (
	ErrEmptyTitle       = errors.New("post: title must not be empty")
	ErrEmptyBody        = errors.New("post: body must not be empty")
	ErrCommentsDisabled = errors.New("post: comments are disabled for this post")
	ErrNotFound         = errors.New("post: not found")
)
