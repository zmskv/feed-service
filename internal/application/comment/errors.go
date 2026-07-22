package comment

import "errors"

var ErrParentMismatch = errors.New("comment: parent comment belongs to a different post")
