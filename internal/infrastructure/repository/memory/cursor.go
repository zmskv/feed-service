package memory

import (
	"time"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/pagination"
)

func isOlderThanCursor(createdAt time.Time, id uuid.UUID, c pagination.Cursor) bool {
	if createdAt.Equal(c.CreatedAt) {
		return id.String() < c.ID.String()
	}
	return createdAt.Before(c.CreatedAt)
}

func isNewerThanCursor(createdAt time.Time, id uuid.UUID, c pagination.Cursor) bool {
	if createdAt.Equal(c.CreatedAt) {
		return id.String() > c.ID.String()
	}
	return createdAt.After(c.CreatedAt)
}
