package graphql

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"

	appComment "github.com/zmskv/feed-service/internal/application/comment"
	"github.com/zmskv/feed-service/internal/pagination"
)

type commentLoader = dataloader.Loader[uuid.UUID, *appComment.Page]

type pageArgs struct {
	first    int
	hasAfter bool
	afterAt  time.Time
	afterID  uuid.UUID
}

func argsFor(first int, after *pagination.Cursor) pageArgs {
	if after == nil {
		return pageArgs{first: first}
	}
	return pageArgs{first: first, hasAfter: true, afterAt: after.CreatedAt, afterID: after.ID}
}

type Loaders struct {
	comments CommentService

	mu            sync.Mutex
	topLevel      map[pageArgs]*commentLoader
	repliesByArgs map[pageArgs]*commentLoader
}

func newLoaders(comments CommentService) *Loaders {
	return &Loaders{
		comments:      comments,
		topLevel:      make(map[pageArgs]*commentLoader),
		repliesByArgs: make(map[pageArgs]*commentLoader),
	}
}

func (l *Loaders) topLevelLoader(first int, after *pagination.Cursor) *commentLoader {
	return getOrCreate(&l.mu, l.topLevel, argsFor(first, after), func() *commentLoader {
		return dataloader.NewBatchedLoader(func(ctx context.Context, postIDs []uuid.UUID) []*dataloader.Result[*appComment.Page] {
			pages, err := l.comments.ListByPostsBatch(ctx, postIDs, first, after)
			return toResults(postIDs, pages, err)
		})
	})
}

func (l *Loaders) repliesLoader(first int, after *pagination.Cursor) *commentLoader {
	return getOrCreate(&l.mu, l.repliesByArgs, argsFor(first, after), func() *commentLoader {
		return dataloader.NewBatchedLoader(func(ctx context.Context, parentIDs []uuid.UUID) []*dataloader.Result[*appComment.Page] {
			pages, err := l.comments.ListRepliesByParentsBatch(ctx, parentIDs, first, after)
			return toResults(parentIDs, pages, err)
		})
	})
}

func toResults(keys []uuid.UUID, pages map[uuid.UUID]*appComment.Page, err error) []*dataloader.Result[*appComment.Page] {
	results := make([]*dataloader.Result[*appComment.Page], len(keys))
	for i, key := range keys {
		if err != nil {
			results[i] = &dataloader.Result[*appComment.Page]{Error: err}
			continue
		}
		page := pages[key]
		if page == nil {
			page = &appComment.Page{}
		}
		results[i] = &dataloader.Result[*appComment.Page]{Data: page}
	}
	return results
}

func getOrCreate(mu *sync.Mutex, m map[pageArgs]*commentLoader, key pageArgs, create func() *commentLoader) *commentLoader {
	mu.Lock()
	defer mu.Unlock()
	if l, ok := m[key]; ok {
		return l
	}
	l := create()
	m[key] = l
	return l
}

type loadersCtxKey struct{}

func Middleware(comments CommentService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), loadersCtxKey{}, newLoaders(comments))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func loadersFrom(ctx context.Context) *Loaders {
	return ctx.Value(loadersCtxKey{}).(*Loaders)
}
