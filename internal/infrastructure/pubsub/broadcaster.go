package pubsub

import (
	"sync"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/domain/comment"
)

type Broadcaster struct {
	mu   sync.Mutex
	subs map[uuid.UUID]map[chan *comment.Comment]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: make(map[uuid.UUID]map[chan *comment.Comment]struct{})}
}

func (b *Broadcaster) Publish(c *comment.Comment) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.subs[c.PostID] {
		select {
		case ch <- c:
		default:
		}
	}
}

func (b *Broadcaster) Subscribe(postID uuid.UUID) (<-chan *comment.Comment, func()) {
	ch := make(chan *comment.Comment, 1)

	b.mu.Lock()
	if b.subs[postID] == nil {
		b.subs[postID] = make(map[chan *comment.Comment]struct{})
	}
	b.subs[postID][ch] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[postID], ch)
		if len(b.subs[postID]) == 0 {
			delete(b.subs, postID)
		}
		close(ch)
	}
	return ch, unsubscribe
}
