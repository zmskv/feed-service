package graphql_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type wsMsg struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func TestSubscription_CommentAdded(t *testing.T) {
	baseURL := newTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	postID := graphqlRequest(t, baseURL, `mutation {
		createPost(input: {authorId: "22222222-2222-2222-2222-222222222222", title: "t", body: "b"}) { id }
	}`, "createPost", "id")

	wsURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/query"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Subprotocols: []string{"graphql-transport-ws"},
	})
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	wsSend(ctx, t, conn, wsMsg{Type: "connection_init"})
	if ack := wsRecv(ctx, t, conn); ack.Type != "connection_ack" {
		t.Fatalf("expected connection_ack, got %q", ack.Type)
	}

	subPayload, _ := json.Marshal(map[string]string{
		"query": fmt.Sprintf(`subscription { commentAdded(postId: %q) { id body } }`, postID),
	})
	wsSend(ctx, t, conn, wsMsg{ID: "1", Type: "subscribe", Payload: subPayload})

	time.Sleep(100 * time.Millisecond)

	commentID := graphqlRequest(t, baseURL, fmt.Sprintf(`mutation {
		createComment(input: {postId: %q, authorId: "44444444-4444-4444-4444-444444444444", body: "pushed"}) { id }
	}`, postID), "createComment", "id")

	for {
		msg := wsRecv(ctx, t, conn)
		switch msg.Type {
		case "next":
			var payload struct {
				Data struct {
					CommentAdded struct {
						ID   string `json:"id"`
						Body string `json:"body"`
					} `json:"commentAdded"`
				} `json:"data"`
			}
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				t.Fatalf("unmarshal push payload: %v (%s)", err, msg.Payload)
			}
			if payload.Data.CommentAdded.ID != commentID {
				t.Fatalf("pushed comment id = %s, want %s", payload.Data.CommentAdded.ID, commentID)
			}
			if payload.Data.CommentAdded.Body != "pushed" {
				t.Fatalf("pushed comment body = %q, want %q", payload.Data.CommentAdded.Body, "pushed")
			}
			return
		case "error":
			t.Fatalf("subscription error: %s", msg.Payload)
		default:
		}
	}
}

func wsSend(ctx context.Context, t *testing.T, conn *websocket.Conn, m wsMsg) {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

func wsRecv(ctx context.Context, t *testing.T, conn *websocket.Conn) wsMsg {
	t.Helper()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("websocket read: %v", err)
	}
	var m wsMsg
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal ws message: %v (%s)", err, data)
	}
	return m
}
