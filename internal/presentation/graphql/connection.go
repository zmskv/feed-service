package graphql

import (
	"github.com/google/uuid"

	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	domainPost "github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
	"github.com/zmskv/feed-service/internal/presentation/graphql/generated"
)

const postsScope = "posts"

func commentsScope(postID uuid.UUID) string  { return "comments:" + postID.String() }
func repliesScope(parentID uuid.UUID) string { return "replies:" + parentID.String() }

func decodeCursor(scope string, after *string) (*pagination.Cursor, error) {
	if after == nil {
		return nil, nil
	}
	c, err := pagination.Decode(scope, *after)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func buildPageInfo(cursors []string, hasNext bool) *generated.PageInfo {
	pageInfo := &generated.PageInfo{HasNextPage: hasNext}
	if len(cursors) > 0 {
		endCursor := cursors[len(cursors)-1]
		pageInfo.EndCursor = &endCursor
	}
	return pageInfo
}

func buildPostConnection(items []*domainPost.Post, hasNext bool) *generated.PostConnection {
	edges := make([]*generated.PostEdge, len(items))
	cursors := make([]string, len(items))
	for i, p := range items {
		cursor := pagination.Encode(postsScope, pagination.Cursor{CreatedAt: p.CreatedAt, ID: p.ID})
		edges[i] = &generated.PostEdge{Node: postToModel(p), Cursor: cursor}
		cursors[i] = cursor
	}
	return &generated.PostConnection{Edges: edges, PageInfo: buildPageInfo(cursors, hasNext)}
}

func buildCommentConnection(scope string, items []*domainComment.Comment, hasNext bool) *generated.CommentConnection {
	edges := make([]*generated.CommentEdge, len(items))
	cursors := make([]string, len(items))
	for i, c := range items {
		cursor := pagination.Encode(scope, pagination.Cursor{CreatedAt: c.CreatedAt, ID: c.ID})
		edges[i] = &generated.CommentEdge{Node: commentToModel(c), Cursor: cursor}
		cursors[i] = cursor
	}
	return &generated.CommentConnection{Edges: edges, PageInfo: buildPageInfo(cursors, hasNext)}
}
