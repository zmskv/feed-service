package pagination

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
)

func base64Encode(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}

func TestEncodeDecode_RoundTrip(t *testing.T) {
	want := Cursor{CreatedAt: time.Now().Truncate(time.Nanosecond), ID: uuid.New()}

	got, err := Decode(Encode(want))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) || got.ID != want.ID {
		t.Fatalf("Decode(Encode(c)) = %+v, want %+v", got, want)
	}
}

func TestDecode_Invalid(t *testing.T) {
	tests := []string{"", "not-base64!!!", base64Encode("no-separator"), base64Encode("abc|def")}

	for _, s := range tests {
		if _, err := Decode(s); err == nil {
			t.Fatalf("Decode(%q) = nil error, want ErrInvalidCursor", s)
		}
	}
}

func TestNormalizeFirst(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{0, DefaultPageSize},
		{-5, DefaultPageSize},
		{10, 10},
		{MaxPageSize + 50, MaxPageSize},
	}

	for _, tt := range tests {
		if got := NormalizeFirst(tt.in); got != tt.want {
			t.Errorf("NormalizeFirst(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
