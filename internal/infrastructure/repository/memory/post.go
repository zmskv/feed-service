package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
)

type Post struct {
	mu    sync.RWMutex
	posts map[uuid.UUID]*post.Post
}

func NewPost() *Post {
	return &Post{posts: make(map[uuid.UUID]*post.Post)}
}

func (r *Post) Save(_ context.Context, p *post.Post) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.posts[p.ID] = p
	return nil
}

func (r *Post) FindByID(_ context.Context, id uuid.UUID) (*post.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.posts[id]
	if !ok {
		return nil, post.ErrNotFound
	}
	return p, nil
}

func (r *Post) List(_ context.Context, first int, after *pagination.Cursor) ([]*post.Post, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]*post.Post, 0, len(r.posts))
	for _, p := range r.posts {
		all = append(all, p)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID.String() > all[j].ID.String()
		}
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	start := 0
	if after != nil {
		start = len(all)
		for i, p := range all {
			if isOlderThanCursor(p.CreatedAt, p.ID, *after) {
				start = i
				break
			}
		}
	}

	end := start + first
	hasNext := end < len(all)
	if end > len(all) {
		end = len(all)
	}

	out := make([]*post.Post, end-start)
	copy(out, all[start:end])
	return out, hasNext, nil
}
