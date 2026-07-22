package comment

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNew(t *testing.T) {
	postID := uuid.New()
	authorID := uuid.New()

	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{"valid", "hello world", nil},
		{"empty body", "", ErrEmptyBody},
		{"exactly max length", strings.Repeat("a", MaxBodyLength), nil},
		{"too long ascii", strings.Repeat("a", MaxBodyLength+1), ErrBodyTooLong},
		{"too long cyrillic", strings.Repeat("я", MaxBodyLength+1), ErrBodyTooLong}, // rune count, not byte count
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(postID, nil, authorID, tt.body)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("New() err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && c.PostID != postID {
				t.Fatalf("PostID = %v, want %v", c.PostID, postID)
			}
		})
	}
}

func TestNew_Reply(t *testing.T) {
	postID := uuid.New()
	authorID := uuid.New()
	parentID := uuid.New()

	c, err := New(postID, &parentID, authorID, "a reply")
	if err != nil {
		t.Fatal(err)
	}
	if c.ParentID == nil || *c.ParentID != parentID {
		t.Fatalf("ParentID = %v, want %v", c.ParentID, parentID)
	}
}
