package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/zmskv/feed-service/internal/domain/comment"
	"github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/infrastructure/repository/postgres"
	"github.com/zmskv/feed-service/internal/pagination"
)

func seedPost(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	p, err := post.New(uuid.New(), "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.NewPost(db).Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.ExecContext(context.Background(), `DELETE FROM posts WHERE id = $1`, p.ID) })
	return p.ID
}

func TestComment_SaveFindByID(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	postID := seedPost(t, db)
	repo := postgres.NewComment(db)

	c, err := comment.New(postID, nil, uuid.New(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, c); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.FindByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got.ID != c.ID || got.Body != c.Body {
		t.Fatalf("FindByID() = %+v, want %+v", got, c)
	}

	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(err, comment.ErrNotFound) {
		t.Fatalf("FindByID() missing = %v, want %v", err, comment.ErrNotFound)
	}
}

func TestComment_ListByParent_TopLevelVsReplies(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	postID := seedPost(t, db)
	otherPostID := seedPost(t, db)
	repo := postgres.NewComment(db)

	top, err := comment.New(postID, nil, uuid.New(), "top")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, top); err != nil {
		t.Fatal(err)
	}

	reply, err := comment.New(postID, &top.ID, uuid.New(), "reply")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, reply); err != nil {
		t.Fatal(err)
	}

	elsewhere, err := comment.New(otherPostID, nil, uuid.New(), "elsewhere")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, elsewhere); err != nil {
		t.Fatal(err)
	}

	topLevel, _, err := repo.ListByParent(ctx, postID, nil, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(topLevel) != 1 || topLevel[0].ID != top.ID {
		t.Fatalf("ListByParent(top-level) = %+v, want only %v", topLevel, top.ID)
	}

	replies, _, err := repo.ListByParent(ctx, postID, &top.ID, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 1 || replies[0].ID != reply.ID {
		t.Fatalf("ListByParent(replies) = %+v, want only %v", replies, reply.ID)
	}
}

func TestComment_ListByParent_OldestFirstAndPagination(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	postID := seedPost(t, db)
	repo := postgres.NewComment(db)

	const total = 13
	var order []uuid.UUID
	for i := 0; i < total; i++ {
		c, err := comment.New(postID, nil, uuid.New(), "hi")
		if err != nil {
			t.Fatal(err)
		}
		c.CreatedAt = c.CreatedAt.Add(time.Duration(i) * time.Second)
		if err := repo.Save(ctx, c); err != nil {
			t.Fatal(err)
		}
		order = append(order, c.ID)
	}

	var got []uuid.UUID
	var cursor *pagination.Cursor
	for {
		items, hasNext, err := repo.ListByParent(ctx, postID, nil, 4, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range items {
			got = append(got, c.ID)
		}
		if !hasNext {
			break
		}
		last := items[len(items)-1]
		cursor = &pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}

	if len(got) != total {
		t.Fatalf("got %d comments, want %d", len(got), total)
	}
	for i, id := range order {
		if got[i] != id {
			t.Fatalf("order[%d] = %v, want %v (oldest-first)", i, got[i], id)
		}
	}
}

func TestComment_ListTopLevelByPosts_TruncatesInRankOrder(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := postgres.NewComment(db)

	const perPost = 7
	const first = 5
	postA := seedPost(t, db)
	postB := seedPost(t, db)

	base := time.Now().UTC()
	wantOldestFirst := map[uuid.UUID][]uuid.UUID{postA: nil, postB: nil}
	for i := perPost - 1; i >= 0; i-- {
		for _, postID := range []uuid.UUID{postA, postB} {
			c, err := comment.New(postID, nil, uuid.New(), "c")
			if err != nil {
				t.Fatal(err)
			}
			c.CreatedAt = base.Add(time.Duration(i) * time.Second)
			if err := repo.Save(ctx, c); err != nil {
				t.Fatal(err)
			}
			wantOldestFirst[postID] = append([]uuid.UUID{c.ID}, wantOldestFirst[postID]...)
		}
	}

	pages, err := repo.ListTopLevelByPosts(ctx, []uuid.UUID{postA, postB}, first, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, postID := range []uuid.UUID{postA, postB} {
		p := pages[postID]
		if p == nil {
			t.Fatalf("post %v: no page returned", postID)
		}
		if !p.HasNext {
			t.Fatalf("post %v: hasNext = false, want true (%d comments, first=%d)", postID, perPost, first)
		}
		if len(p.Items) != first {
			t.Fatalf("post %v: got %d items, want %d", postID, len(p.Items), first)
		}
		want := wantOldestFirst[postID][:first]
		for i, c := range p.Items {
			if c.ID != want[i] {
				t.Fatalf("post %v item[%d] = %v, want %v (oldest-first, rank order) — got %v, want %v",
					postID, i, c.ID, want[i], idsOf(p.Items), want)
			}
		}
	}
}

func idsOf(cs []*comment.Comment) []uuid.UUID {
	out := make([]uuid.UUID, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}
