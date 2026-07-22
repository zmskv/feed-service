package memory

import (
	"github.com/zmskv/feed-service/internal/application/comment"
	"github.com/zmskv/feed-service/internal/application/post"
)

// compile assertion 
var (
	_ post.Repository    = (*Post)(nil) 
	_ comment.PostRepository = (*Post)(nil)

	_ comment.Repository = (*Comment)(nil)
)
