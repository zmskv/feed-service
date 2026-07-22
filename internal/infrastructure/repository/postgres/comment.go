package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	appComment "github.com/zmskv/feed-service/internal/application/comment"
	"github.com/zmskv/feed-service/internal/domain/comment"
	"github.com/zmskv/feed-service/internal/pagination"
)

type Comment struct {
	db *sqlx.DB
}

func NewComment(db *sqlx.DB) *Comment {
	return &Comment{db: db}
}

type commentRow struct {
	ID        uuid.UUID  `db:"id"`
	PostID    uuid.UUID  `db:"post_id"`
	ParentID  *uuid.UUID `db:"parent_id"`
	AuthorID  uuid.UUID  `db:"author_id"`
	Body      string     `db:"body"`
	CreatedAt time.Time  `db:"created_at"`
}

func (r commentRow) toDomain() *comment.Comment {
	return &comment.Comment{
		ID:        r.ID,
		PostID:    r.PostID,
		ParentID:  r.ParentID,
		AuthorID:  r.AuthorID,
		Body:      r.Body,
		CreatedAt: r.CreatedAt,
	}
}

func (r *Comment) Save(ctx context.Context, c *comment.Comment) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO comments (id, post_id, parent_id, author_id, body, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		c.ID, c.PostID, c.ParentID, c.AuthorID, c.Body, c.CreatedAt)
	return err
}

func (r *Comment) FindByID(ctx context.Context, id uuid.UUID) (*comment.Comment, error) {
	var row commentRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM comments WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, comment.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *Comment) ListByParent(ctx context.Context, postID uuid.UUID, parentID *uuid.UUID, first int, after *pagination.Cursor) ([]*comment.Comment, bool, error) {
	var rows []commentRow
	var err error

	switch {
	case parentID == nil && after == nil:
		err = r.db.SelectContext(ctx, &rows, `
			SELECT * FROM comments
			WHERE post_id = $1 AND parent_id IS NULL
			ORDER BY created_at, id
			LIMIT $2`, postID, first+1)
	case parentID == nil && after != nil:
		err = r.db.SelectContext(ctx, &rows, `
			SELECT * FROM comments
			WHERE post_id = $1 AND parent_id IS NULL AND (created_at, id) > ($2, $3)
			ORDER BY created_at, id
			LIMIT $4`, postID, after.CreatedAt, after.ID, first+1)
	case parentID != nil && after == nil:
		err = r.db.SelectContext(ctx, &rows, `
			SELECT * FROM comments
			WHERE post_id = $1 AND parent_id = $2
			ORDER BY created_at, id
			LIMIT $3`, postID, *parentID, first+1)
	default:
		err = r.db.SelectContext(ctx, &rows, `
			SELECT * FROM comments
			WHERE post_id = $1 AND parent_id = $2 AND (created_at, id) > ($3, $4)
			ORDER BY created_at, id
			LIMIT $5`, postID, *parentID, after.CreatedAt, after.ID, first+1)
	}
	if err != nil {
		return nil, false, err
	}

	hasNext := len(rows) > first
	if hasNext {
		rows = rows[:first]
	}

	out := make([]*comment.Comment, len(rows))
	for i, row := range rows {
		out[i] = row.toDomain()
	}
	return out, hasNext, nil
}

type rankedCommentRow struct {
	commentRow
	Rn int64 `db:"rn"`
}

func groupRankedRows[K comparable](rows []rankedCommentRow, keys []K, keyOf func(commentRow) K, first int) map[K]*appComment.Page {
	grouped := make(map[K][]rankedCommentRow, len(keys))
	for _, row := range rows {
		k := keyOf(row.commentRow)
		grouped[k] = append(grouped[k], row)
	}

	out := make(map[K]*appComment.Page, len(keys))
	for _, k := range keys {
		group := grouped[k]
		hasNext := len(group) > first
		if hasNext {
			group = group[:first]
		}
		items := make([]*comment.Comment, len(group))
		for i, row := range group {
			items[i] = row.commentRow.toDomain()
		}
		out[k] = &appComment.Page{Items: items, HasNext: hasNext}
	}
	return out
}

func (r *Comment) ListTopLevelByPosts(ctx context.Context, postIDs []uuid.UUID, first int, after *pagination.Cursor) (map[uuid.UUID]*appComment.Page, error) {
	if len(postIDs) == 0 {
		return map[uuid.UUID]*appComment.Page{}, nil
	}

	var rows []rankedCommentRow
	var err error
	if after == nil {
		err = r.db.SelectContext(ctx, &rows, `
			WITH ranked AS (
				SELECT *, ROW_NUMBER() OVER (PARTITION BY post_id ORDER BY created_at, id) AS rn
				FROM comments
				WHERE post_id = ANY($1) AND parent_id IS NULL
			)
			SELECT * FROM ranked WHERE rn <= $2`, postIDs, first+1)
	} else {
		err = r.db.SelectContext(ctx, &rows, `
			WITH ranked AS (
				SELECT *, ROW_NUMBER() OVER (PARTITION BY post_id ORDER BY created_at, id) AS rn
				FROM comments
				WHERE post_id = ANY($1) AND parent_id IS NULL AND (created_at, id) > ($2, $3)
			)
			SELECT * FROM ranked WHERE rn <= $4`, postIDs, after.CreatedAt, after.ID, first+1)
	}
	if err != nil {
		return nil, err
	}

	return groupRankedRows(rows, postIDs, func(c commentRow) uuid.UUID { return c.PostID }, first), nil
}

func (r *Comment) ListRepliesByParents(ctx context.Context, parentIDs []uuid.UUID, first int, after *pagination.Cursor) (map[uuid.UUID]*appComment.Page, error) {
	if len(parentIDs) == 0 {
		return map[uuid.UUID]*appComment.Page{}, nil
	}

	var rows []rankedCommentRow
	var err error
	if after == nil {
		err = r.db.SelectContext(ctx, &rows, `
			WITH ranked AS (
				SELECT *, ROW_NUMBER() OVER (PARTITION BY parent_id ORDER BY created_at, id) AS rn
				FROM comments
				WHERE parent_id = ANY($1)
			)
			SELECT * FROM ranked WHERE rn <= $2`, parentIDs, first+1)
	} else {
		err = r.db.SelectContext(ctx, &rows, `
			WITH ranked AS (
				SELECT *, ROW_NUMBER() OVER (PARTITION BY parent_id ORDER BY created_at, id) AS rn
				FROM comments
				WHERE parent_id = ANY($1) AND (created_at, id) > ($2, $3)
			)
			SELECT * FROM ranked WHERE rn <= $4`, parentIDs, after.CreatedAt, after.ID, first+1)
	}
	if err != nil {
		return nil, err
	}

	return groupRankedRows(rows, parentIDs, func(c commentRow) uuid.UUID {
		if c.ParentID == nil {
			return uuid.Nil
		}
		return *c.ParentID
	}, first), nil
}
