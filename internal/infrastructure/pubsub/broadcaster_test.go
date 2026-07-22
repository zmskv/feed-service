package pubsub_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zmskv/feed-service/internal/domain/comment"
	"github.com/zmskv/feed-service/internal/infrastructure/pubsub"
)

func TestBroadcaster_DeliversOnlyToMatchingPost(t *testing.T) {
	b := pubsub.NewBroadcaster()
	postA, postB := uuid.New(), uuid.New()

	chA, unsubA := b.Subscribe(postA)
	defer unsubA()
	chB, unsubB := b.Subscribe(postB)
	defer unsubB()

	c, err := comment.New(postA, nil, uuid.New(), "hello A")
	if err != nil {
		t.Fatal(err)
	}
	b.Publish(c)

	select {
	case got := <-chA:
		if got.ID != c.ID {
			t.Fatalf("chA got %v, want %v", got.ID, c.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber for postA never received the comment")
	}

	select {
	case got := <-chB:
		t.Fatalf("subscriber for postB should not have received anything, got %+v", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBroadcaster_MultipleSubscribersSamePost(t *testing.T) {
	b := pubsub.NewBroadcaster()
	postID := uuid.New()

	ch1, unsub1 := b.Subscribe(postID)
	defer unsub1()
	ch2, unsub2 := b.Subscribe(postID)
	defer unsub2()

	c, err := comment.New(postID, nil, uuid.New(), "fan-out")
	if err != nil {
		t.Fatal(err)
	}
	b.Publish(c)

	for i, ch := range []<-chan *comment.Comment{ch1, ch2} {
		select {
		case got := <-ch:
			if got.ID != c.ID {
				t.Fatalf("subscriber %d got %v, want %v", i, got.ID, c.ID)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d never received the comment", i)
		}
	}
}

func TestBroadcaster_UnsubscribeStopsDelivery(t *testing.T) {
	b := pubsub.NewBroadcaster()
	postID := uuid.New()

	ch, unsub := b.Subscribe(postID)
	unsub()

	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after unsubscribe")
	}

	c, err := comment.New(postID, nil, uuid.New(), "after unsubscribe")
	if err != nil {
		t.Fatal(err)
	}
	b.Publish(c)
}

func TestBroadcaster_PublishWithNoSubscribersDoesNotBlock(t *testing.T) {
	b := pubsub.NewBroadcaster()
	c, err := comment.New(uuid.New(), nil, uuid.New(), "nobody listening")
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		b.Publish(c)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked with no subscribers")
	}
}
