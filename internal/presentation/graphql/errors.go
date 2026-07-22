package graphql

import (
	"context"
	"errors"

	gql "github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	appComment "github.com/zmskv/feed-service/internal/application/comment"
	appPost "github.com/zmskv/feed-service/internal/application/post"
	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	domainPost "github.com/zmskv/feed-service/internal/domain/post"
)

var errorCodes = []struct {
	err  error
	code string
}{
	{domainPost.ErrEmptyTitle, "EMPTY_TITLE"},
	{domainPost.ErrEmptyBody, "EMPTY_BODY"},
	{domainPost.ErrCommentsDisabled, "COMMENTS_DISABLED"},
	{domainPost.ErrNotFound, "POST_NOT_FOUND"},
	{appPost.ErrForbidden, "FORBIDDEN"},
	{domainComment.ErrEmptyBody, "COMMENT_EMPTY_BODY"},
	{domainComment.ErrBodyTooLong, "BODY_TOO_LONG"},
	{domainComment.ErrNotFound, "COMMENT_NOT_FOUND"},
	{appComment.ErrParentMismatch, "PARENT_MISMATCH"},
}

func ErrorPresenter(ctx context.Context, err error) *gqlerror.Error {
	gqlErr := gql.DefaultErrorPresenter(ctx, err)

	for _, ec := range errorCodes {
		if errors.Is(err, ec.err) {
			if gqlErr.Extensions == nil {
				gqlErr.Extensions = map[string]any{}
			}
			gqlErr.Extensions["code"] = ec.code
			break
		}
	}
	return gqlErr
}
