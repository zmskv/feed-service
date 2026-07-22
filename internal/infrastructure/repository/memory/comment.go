package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/domain/comment"
	"github.com/zmskv/feed-service/internal/pagination"
)

type Comment struct {
	mu       sync.RWMutex
	comments map[uuid.UUID]*comment.Comment
}

func NewComment() *Comment {
	return &Comment{comments: make(map[uuid.UUID]*comment.Comment)}
}

func (r *Comment) Save(_ context.Context, c *comment.Comment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.comments[c.ID] = c
	return nil
}

func (r *Comment) FindByID(_ context.Context, id uuid.UUID) (*comment.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.comments[id]
	if !ok {
		return nil, comment.ErrNotFound
	}
	return c, nil
}

func (r *Comment) ListByParent(_ context.Context, postID uuid.UUID, parentID *uuid.UUID, first int, after *pagination.Cursor) ([]*comment.Comment, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []*comment.Comment
	for _, c := range r.comments {
		if c.PostID != postID {
			continue
		}
		if (c.ParentID == nil) != (parentID == nil) {
			continue
		}
		if c.ParentID != nil && parentID != nil && *c.ParentID != *parentID {
			continue
		}
		matched = append(matched, c)
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID.String() < matched[j].ID.String()
		}
		return matched[i].CreatedAt.Before(matched[j].CreatedAt)
	})

	start := 0
	if after != nil {
		start = len(matched)
		for i, c := range matched {
			if isNewerThanCursor(c.CreatedAt, c.ID, *after) {
				start = i
				break
			}
		}
	}

	end := start + first
	hasNext := end < len(matched)
	if end > len(matched) {
		end = len(matched)
	}

	out := make([]*comment.Comment, end-start)
	copy(out, matched[start:end])
	return out, hasNext, nil
}
