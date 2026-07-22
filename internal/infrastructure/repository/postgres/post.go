package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/zmskv/feed-service/internal/domain/post"
	"github.com/zmskv/feed-service/internal/pagination"
)

type Post struct {
	db *sqlx.DB
}

func NewPost(db *sqlx.DB) *Post {
	return &Post{db: db}
}

type postRow struct {
	ID               uuid.UUID `db:"id"`
	AuthorID         uuid.UUID `db:"author_id"`
	Title            string    `db:"title"`
	Body             string    `db:"body"`
	CommentsDisabled bool      `db:"comments_disabled"`
	CreatedAt        time.Time `db:"created_at"`
}

func (r postRow) toDomain() *post.Post {
	return &post.Post{
		ID:               r.ID,
		AuthorID:         r.AuthorID,
		Title:            r.Title,
		Body:             r.Body,
		CommentsDisabled: r.CommentsDisabled,
		CreatedAt:        r.CreatedAt,
	}
}

func (r *Post) Save(ctx context.Context, p *post.Post) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO posts (id, author_id, title, body, comments_disabled, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			body = EXCLUDED.body,
			comments_disabled = EXCLUDED.comments_disabled`,
		p.ID, p.AuthorID, p.Title, p.Body, p.CommentsDisabled, p.CreatedAt)
	return err
}

func (r *Post) FindByID(ctx context.Context, id uuid.UUID) (*post.Post, error) {
	var row postRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM posts WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, post.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *Post) List(ctx context.Context, first int, after *pagination.Cursor) ([]*post.Post, bool, error) {
	var rows []postRow
	var err error

	if after == nil {
		err = r.db.SelectContext(ctx, &rows, `
			SELECT * FROM posts
			ORDER BY created_at DESC, id DESC
			LIMIT $1`, first+1)
	} else {
		err = r.db.SelectContext(ctx, &rows, `
			SELECT * FROM posts
			WHERE (created_at, id) < ($1, $2)
			ORDER BY created_at DESC, id DESC
			LIMIT $3`, after.CreatedAt, after.ID, first+1)
	}
	if err != nil {
		return nil, false, err
	}

	hasNext := len(rows) > first
	if hasNext {
		rows = rows[:first]
	}

	out := make([]*post.Post, len(rows))
	for i, row := range rows {
		out[i] = row.toDomain()
	}
	return out, hasNext, nil
}
