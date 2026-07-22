package graphql

import (
	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	domainPost "github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
	"github.com/zmskv/feed-service/internal/presentation/graphql/generated"
)

func decodeCursor(after *string) (*pagination.Cursor, error) {
	if after == nil {
		return nil, nil
	}
	c, err := pagination.Decode(*after)
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
		cursor := pagination.Encode(pagination.Cursor{CreatedAt: p.CreatedAt, ID: p.ID})
		edges[i] = &generated.PostEdge{Node: postToModel(p), Cursor: cursor}
		cursors[i] = cursor
	}
	return &generated.PostConnection{Edges: edges, PageInfo: buildPageInfo(cursors, hasNext)}
}

func buildCommentConnection(items []*domainComment.Comment, hasNext bool) *generated.CommentConnection {
	edges := make([]*generated.CommentEdge, len(items))
	cursors := make([]string, len(items))
	for i, c := range items {
		cursor := pagination.Encode(pagination.Cursor{CreatedAt: c.CreatedAt, ID: c.ID})
		edges[i] = &generated.CommentEdge{Node: commentToModel(c), Cursor: cursor}
		cursors[i] = cursor
	}
	return &generated.CommentConnection{Edges: edges, PageInfo: buildPageInfo(cursors, hasNext)}
}
