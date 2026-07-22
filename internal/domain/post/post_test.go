package post

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNew(t *testing.T) {
	authorID := uuid.New()

	tests := []struct {
		name    string
		title   string
		body    string
		wantErr error
	}{
		{"valid", "title", "body", nil},
		{"empty title", "", "body", ErrEmptyTitle},
		{"empty body", "title", "", ErrEmptyBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(authorID, tt.title, tt.body)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("New() err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && p.CommentsDisabled {
				t.Fatalf("new post should not have comments disabled by default")
			}
		})
	}
}

func TestCanAcceptComments(t *testing.T) {
	p, err := New(uuid.New(), "title", "body")
	if err != nil {
		t.Fatal(err)
	}

	if err := p.CanAcceptComments(); err != nil {
		t.Fatalf("CanAcceptComments() = %v, want nil", err)
	}

	p.DisableComments()

	if err := p.CanAcceptComments(); !errors.Is(err, ErrCommentsDisabled) {
		t.Fatalf("CanAcceptComments() = %v, want %v", err, ErrCommentsDisabled)
	}
}
