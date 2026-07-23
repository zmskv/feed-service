package graphql

import (
	"context"
	"errors"

	gql "github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.uber.org/zap"

	appComment "github.com/zmskv/feed-service/internal/application/comment"
	appPost "github.com/zmskv/feed-service/internal/application/post"
	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	domainPost "github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
)

var ErrInvalidID = errors.New("graphql: invalid id")

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, ErrInvalidID
	}
	return id, nil
}

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
	{pagination.ErrInvalidCursor, "INVALID_CURSOR"},
	{ErrInvalidID, "INVALID_ID"},
}

func NewErrorPresenter(log *zap.Logger) gql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		gqlErr := gql.DefaultErrorPresenter(ctx, err)
		if gqlErr.Err == nil {
			return gqlErr
		}

		for _, ec := range errorCodes {
			if errors.Is(gqlErr.Err, ec.err) {
				if gqlErr.Extensions == nil {
					gqlErr.Extensions = map[string]any{}
				}
				gqlErr.Extensions["code"] = ec.code
				return gqlErr
			}
		}

		log.Error("unhandled graphql resolver error", zap.Error(gqlErr.Err), zap.Any("path", gqlErr.Path))
		return &gqlerror.Error{
			Message:   "internal server error",
			Path:      gqlErr.Path,
			Locations: gqlErr.Locations,
			Extensions: map[string]any{
				"code": "INTERNAL_ERROR",
			},
		}
	}
}
