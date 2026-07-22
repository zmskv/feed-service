package post

import "errors"

var ErrForbidden = errors.New("post: only the author can disable comments")
