package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"

	appComment "github.com/zmskv/feed-service/internal/application/comment"
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
	stored := *c
	r.comments[c.ID] = &stored
	return nil
}

func (r *Comment) FindByID(_ context.Context, id uuid.UUID) (*comment.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.comments[id]
	if !ok {
		return nil, comment.ErrNotFound
	}
	out := *c
	return &out, nil
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
		cp := *c
		matched = append(matched, &cp)
	}

	items, hasNext := paginateComments(matched, first, after)
	return items, hasNext, nil
}

func (r *Comment) ListTopLevelByPosts(_ context.Context, postIDs []uuid.UUID, first int, after *pagination.Cursor) (map[uuid.UUID]*appComment.Page, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	want := make(map[uuid.UUID]bool, len(postIDs))
	for _, id := range postIDs {
		want[id] = true
	}

	grouped := make(map[uuid.UUID][]*comment.Comment)
	for _, c := range r.comments {
		if c.ParentID != nil || !want[c.PostID] {
			continue
		}
		cp := *c
		grouped[c.PostID] = append(grouped[c.PostID], &cp)
	}

	out := make(map[uuid.UUID]*appComment.Page, len(postIDs))
	for _, postID := range postIDs {
		items, hasNext := paginateComments(grouped[postID], first, after)
		out[postID] = &appComment.Page{Items: items, HasNext: hasNext}
	}
	return out, nil
}

func (r *Comment) ListRepliesByParents(_ context.Context, parentIDs []uuid.UUID, first int, after *pagination.Cursor) (map[uuid.UUID]*appComment.Page, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	want := make(map[uuid.UUID]bool, len(parentIDs))
	for _, id := range parentIDs {
		want[id] = true
	}

	grouped := make(map[uuid.UUID][]*comment.Comment)
	for _, c := range r.comments {
		if c.ParentID == nil || !want[*c.ParentID] {
			continue
		}
		cp := *c
		grouped[*c.ParentID] = append(grouped[*c.ParentID], &cp)
	}

	out := make(map[uuid.UUID]*appComment.Page, len(parentIDs))
	for _, parentID := range parentIDs {
		items, hasNext := paginateComments(grouped[parentID], first, after)
		out[parentID] = &appComment.Page{Items: items, HasNext: hasNext}
	}
	return out, nil
}

func paginateComments(matched []*comment.Comment, first int, after *pagination.Cursor) ([]*comment.Comment, bool) {
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

	out := make([]*comment.Comment, len(matched[start:end]))
	copy(out, matched[start:end])
	return out, hasNext
}
