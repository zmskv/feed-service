package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

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
