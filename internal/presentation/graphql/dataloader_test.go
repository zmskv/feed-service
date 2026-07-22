package graphql

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	appComment "github.com/zmskv/feed-service/internal/application/comment"
	domainComment "github.com/zmskv/feed-service/internal/domain/comment"
	"github.com/zmskv/feed-service/internal/pagination"
)

type countingCommentService struct {
	mu            sync.Mutex
	topLevelCalls int
	repliesCalls  int
}

func (s *countingCommentService) ListByPostsBatch(_ context.Context, postIDs []uuid.UUID, _ int, _ *pagination.Cursor) (map[uuid.UUID]*appComment.Page, error) {
	s.mu.Lock()
	s.topLevelCalls++
	s.mu.Unlock()

	out := make(map[uuid.UUID]*appComment.Page, len(postIDs))
	for _, id := range postIDs {
		out[id] = &appComment.Page{}
	}
	return out, nil
}

func (s *countingCommentService) ListRepliesByParentsBatch(_ context.Context, parentIDs []uuid.UUID, _ int, _ *pagination.Cursor) (map[uuid.UUID]*appComment.Page, error) {
	s.mu.Lock()
	s.repliesCalls++
	s.mu.Unlock()

	out := make(map[uuid.UUID]*appComment.Page, len(parentIDs))
	for _, id := range parentIDs {
		out[id] = &appComment.Page{}
	}
	return out, nil
}

func (s *countingCommentService) Create(context.Context, uuid.UUID, *uuid.UUID, uuid.UUID, string) (*domainComment.Comment, error) {
	panic("not used by this test")
}

func loadAllTopLevel(t *testing.T, loaders *Loaders, postIDs []uuid.UUID) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make([]error, len(postIDs))
	for i, id := range postIDs {
		wg.Add(1)
		go func(i int, id uuid.UUID) {
			defer wg.Done()
			_, err := loaders.topLevelLoader(10, nil).Load(context.Background(), id)()
			errs[i] = err
		}(i, id)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoaders_BatchesTopLevelAcrossSiblingPosts(t *testing.T) {
	tests := []struct{ numPosts int }{{numPosts: 3}, {numPosts: 50}}

	for _, tt := range tests {
		svc := &countingCommentService{}
		loaders := newLoaders(svc)

		postIDs := make([]uuid.UUID, tt.numPosts)
		for i := range postIDs {
			postIDs[i] = uuid.New()
		}

		loadAllTopLevel(t, loaders, postIDs)

		if svc.topLevelCalls != 1 {
			t.Fatalf("numPosts=%d: ListByPostsBatch called %d times, want 1 (batching should keep it constant, not grow with node count)", tt.numPosts, svc.topLevelCalls)
		}
	}
}

func TestLoaders_BatchesRepliesAcrossSiblingComments(t *testing.T) {
	svc := &countingCommentService{}
	loaders := newLoaders(svc)

	const numComments = 40
	commentIDs := make([]uuid.UUID, numComments)
	for i := range commentIDs {
		commentIDs[i] = uuid.New()
	}

	var wg sync.WaitGroup
	for _, id := range commentIDs {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			if _, err := loaders.repliesLoader(5, nil).Load(context.Background(), id)(); err != nil {
				t.Error(err)
			}
		}(id)
	}
	wg.Wait()

	if svc.repliesCalls != 1 {
		t.Fatalf("ListRepliesByParentsBatch called %d times for %d sibling comments, want 1", svc.repliesCalls, numComments)
	}
}

func TestLoaders_DifferentArgsDontShareABatch(t *testing.T) {
	svc := &countingCommentService{}
	loaders := newLoaders(svc)

	var wg sync.WaitGroup
	for _, first := range []int{5, 10} {
		wg.Add(1)
		go func(first int) {
			defer wg.Done()
			if _, err := loaders.topLevelLoader(first, nil).Load(context.Background(), uuid.New())(); err != nil {
				t.Error(err)
			}
		}(first)
	}
	wg.Wait()

	if svc.topLevelCalls != 2 {
		t.Fatalf("topLevelCalls = %d, want 2 (one per distinct first/after combination)", svc.topLevelCalls)
	}
}
