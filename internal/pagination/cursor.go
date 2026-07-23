package pagination

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidCursor = errors.New("pagination: invalid cursor")

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func Encode(scope string, c Cursor) string {
	raw := scope + "|" + strconv.FormatInt(c.CreatedAt.UnixNano(), 10) + "|" + c.ID.String()
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

func Decode(scope, s string) (Cursor, error) {
	raw, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	gotScope, rest, ok := strings.Cut(string(raw), "|")
	if !ok || gotScope != scope {
		return Cursor{}, ErrInvalidCursor
	}
	nanosStr, idStr, ok := strings.Cut(rest, "|")
	if !ok {
		return Cursor{}, ErrInvalidCursor
	}
	nanos, err := strconv.ParseInt(nanosStr, 10, 64)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	return Cursor{CreatedAt: time.Unix(0, nanos), ID: id}, nil
}

func NormalizeFirst(first int) int {
	if first <= 0 {
		return DefaultPageSize
	}
	if first > MaxPageSize {
		return MaxPageSize
	}
	return first
}
