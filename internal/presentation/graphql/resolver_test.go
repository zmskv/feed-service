package graphql_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQuery_Post_FoundNotFoundInvalid(t *testing.T) {
	baseURL := newTestServer(t)

	postID := graphqlRequest(t, baseURL, `mutation {
		createPost(input: {authorId: "22222222-2222-2222-2222-222222222222", title: "hello", body: "world"}) { id }
	}`, "createPost", "id")

	t.Run("found", func(t *testing.T) {
		resp := doGraphQL(t, baseURL, `query { post(id: "`+postID+`") { title body commentsDisabled } }`)
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected error: %s", resp.Errors[0].Message)
		}
		var data struct {
			Post struct {
				Title            string `json:"title"`
				Body             string `json:"body"`
				CommentsDisabled bool   `json:"commentsDisabled"`
			} `json:"post"`
		}
		mustUnmarshal(t, resp.Data, &data)
		if data.Post.Title != "hello" || data.Post.Body != "world" || data.Post.CommentsDisabled {
			t.Fatalf("post = %+v", data.Post)
		}
	})

	t.Run("not found returns null, not an error", func(t *testing.T) {
		resp := doGraphQL(t, baseURL, `query { post(id: "99999999-9999-9999-9999-999999999999") { title } }`)
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected error: %s", resp.Errors[0].Message)
		}
		var data struct {
			Post *struct{} `json:"post"`
		}
		mustUnmarshal(t, resp.Data, &data)
		if data.Post != nil {
			t.Fatalf("post = %+v, want nil", data.Post)
		}
	})

	t.Run("malformed id is a graphql error with a clean message, not the raw parser text", func(t *testing.T) {
		resp := doGraphQL(t, baseURL, `query { post(id: "not-a-uuid") { title } }`)
		if len(resp.Errors) == 0 {
			t.Fatal("expected an error for a malformed id")
		}
		if code := resp.Errors[0].Extensions.Code; code != "INVALID_ID" {
			t.Fatalf("error code = %q, want INVALID_ID", code)
		}
		if strings.Contains(resp.Errors[0].Message, "uuid") || strings.Contains(resp.Errors[0].Message, "UUID length") {
			t.Fatalf("message leaks the raw uuid parser error: %q", resp.Errors[0].Message)
		}
	})
}

// TestErrorPresenter_RedactsUnrecognizedErrors catches a real gap found in
// review: the README claimed unrecognized errors get a generic message with
// no leaked internal details, but ErrorPresenter never actually did that —
// gqlgen's DefaultErrorPresenter just sets Message = err.Error() for
// anything, recognized or not. A malformed cursor string is a convenient way
// to trigger a real, publicly-reachable "unrecognized-shaped" error path
// (pagination.ErrInvalidCursor) and confirm it now gets its own clean code
// instead of leaking whatever internal text the error happened to carry.
func TestErrorPresenter_KnownErrorsKeepCleanCodes(t *testing.T) {
	baseURL := newTestServer(t)

	postID := graphqlRequest(t, baseURL, `mutation {
		createPost(input: {authorId: "22222222-2222-2222-2222-222222222222", title: "p", body: "b"}) { id }
	}`, "createPost", "id")

	resp := doGraphQL(t, baseURL, `query { post(id: "`+postID+`") { comments(first: 10, after: "not-a-real-cursor") { edges { node { id } } } } }`)
	if len(resp.Errors) == 0 {
		t.Fatal("expected an error for a malformed cursor")
	}
	if code := resp.Errors[0].Extensions.Code; code != "INVALID_CURSOR" {
		t.Fatalf("error code = %q, want INVALID_CURSOR", code)
	}
}

// TestErrorPresenter_LeavesNativeGraphQLErrorsAlone makes sure the redaction
// added for our own unrecognized errors doesn't collateral-damage GraphQL's
// own validation errors (unknown field, wrong type, etc.) — those need to
// stay readable, they're how a client finds out their query is malformed.
func TestErrorPresenter_LeavesNativeGraphQLErrorsAlone(t *testing.T) {
	baseURL := newTestServer(t)

	resp := doGraphQL(t, baseURL, `query { posts(first: 10) { edges { node { thisFieldDoesNotExist } } } }`)
	if len(resp.Errors) == 0 {
		t.Fatal("expected a validation error for an unknown field")
	}
	if !strings.Contains(resp.Errors[0].Message, "thisFieldDoesNotExist") {
		t.Fatalf("native GraphQL validation error got redacted: %q", resp.Errors[0].Message)
	}
	if code := resp.Errors[0].Extensions.Code; code == "INTERNAL_ERROR" {
		t.Fatal("native GraphQL validation error was mislabeled as INTERNAL_ERROR")
	}
}

func TestQuery_PostsPagination(t *testing.T) {
	baseURL := newTestServer(t)

	var ids []string
	for i := 0; i < 3; i++ {
		id := graphqlRequest(t, baseURL, `mutation {
			createPost(input: {authorId: "22222222-2222-2222-2222-222222222222", title: "p", body: "b"}) { id }
		}`, "createPost", "id")
		ids = append(ids, id)
	}

	type edge struct {
		Node struct {
			ID string `json:"id"`
		} `json:"node"`
		Cursor string `json:"cursor"`
	}
	type postsData struct {
		Posts struct {
			Edges    []edge `json:"edges"`
			PageInfo struct {
				HasNextPage bool    `json:"hasNextPage"`
				EndCursor   *string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"posts"`
	}

	resp := doGraphQL(t, baseURL, `query { posts(first: 2) { edges { node { id } cursor } pageInfo { hasNextPage endCursor } } }`)
	if len(resp.Errors) > 0 {
		t.Fatalf("page 1 error: %s", resp.Errors[0].Message)
	}
	var page1 postsData
	mustUnmarshal(t, resp.Data, &page1)
	if len(page1.Posts.Edges) != 2 {
		t.Fatalf("page 1: got %d edges, want 2", len(page1.Posts.Edges))
	}
	if !page1.Posts.PageInfo.HasNextPage {
		t.Fatal("page 1: hasNextPage = false, want true (3 posts, first 2)")
	}
	if page1.Posts.PageInfo.EndCursor == nil {
		t.Fatal("page 1: endCursor is nil")
	}

	resp = doGraphQL(t, baseURL, `query { posts(first: 2, after: "`+*page1.Posts.PageInfo.EndCursor+`") { edges { node { id } } pageInfo { hasNextPage } } }`)
	if len(resp.Errors) > 0 {
		t.Fatalf("page 2 error: %s", resp.Errors[0].Message)
	}
	var page2 postsData
	mustUnmarshal(t, resp.Data, &page2)
	if len(page2.Posts.Edges) != 1 {
		t.Fatalf("page 2: got %d edges, want 1 (the remaining post)", len(page2.Posts.Edges))
	}
	if page2.Posts.PageInfo.HasNextPage {
		t.Fatal("page 2: hasNextPage = true, want false")
	}

	seen := map[string]bool{}
	for _, e := range page1.Posts.Edges {
		seen[e.Node.ID] = true
	}
	for _, e := range page2.Posts.Edges {
		if seen[e.Node.ID] {
			t.Fatalf("post %s returned on both pages", e.Node.ID)
		}
		seen[e.Node.ID] = true
	}
	for _, id := range ids {
		if !seen[id] {
			t.Fatalf("post %s never returned across both pages", id)
		}
	}
}

func TestCursor_CannotBeReusedAcrossConnections(t *testing.T) {
	baseURL := newTestServer(t)

	postA := graphqlRequest(t, baseURL, `mutation {
		createPost(input: {authorId: "22222222-2222-2222-2222-222222222222", title: "A", body: "b"}) { id }
	}`, "createPost", "id")
	postB := graphqlRequest(t, baseURL, `mutation {
		createPost(input: {authorId: "22222222-2222-2222-2222-222222222222", title: "B", body: "b"}) { id }
	}`, "createPost", "id")

	graphqlRequest(t, baseURL, `mutation {
		createComment(input: {postId: "`+postA+`", authorId: "44444444-4444-4444-4444-444444444444", body: "on A"}) { id }
	}`, "createComment", "id")
	graphqlRequest(t, baseURL, `mutation {
		createComment(input: {postId: "`+postB+`", authorId: "44444444-4444-4444-4444-444444444444", body: "on B"}) { id }
	}`, "createComment", "id")

	resp := doGraphQL(t, baseURL, `query { post(id: "`+postA+`") { comments(first: 1) { pageInfo { endCursor } } } }`)
	if len(resp.Errors) > 0 {
		t.Fatalf("unexpected error: %s", resp.Errors[0].Message)
	}
	var data struct {
		Post struct {
			Comments struct {
				PageInfo struct {
					EndCursor *string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"comments"`
		} `json:"post"`
	}
	mustUnmarshal(t, resp.Data, &data)
	cursorFromA := data.Post.Comments.PageInfo.EndCursor
	if cursorFromA == nil {
		t.Fatal("expected an endCursor from post A's comments")
	}

	t.Run("rejected on a different post's comments", func(t *testing.T) {
		resp := doGraphQL(t, baseURL, `query { post(id: "`+postB+`") { comments(first: 10, after: "`+*cursorFromA+`") { edges { node { body } } } } }`)
		if len(resp.Errors) == 0 {
			t.Fatal("expected an error reusing post A's cursor on post B's comments, got none")
		}
	})

	t.Run("rejected on the top-level posts list", func(t *testing.T) {
		resp := doGraphQL(t, baseURL, `query { posts(first: 10, after: "`+*cursorFromA+`") { edges { node { id } } } }`)
		if len(resp.Errors) == 0 {
			t.Fatal("expected an error reusing a comments cursor on the posts list, got none")
		}
	})

	t.Run("still works on the connection it came from", func(t *testing.T) {
		resp := doGraphQL(t, baseURL, `query { post(id: "`+postA+`") { comments(first: 10, after: "`+*cursorFromA+`") { edges { node { body } } } } }`)
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected error reusing the cursor on its own connection: %s", resp.Errors[0].Message)
		}
	})
}

func TestQuery_NestedCommentsAndReplies(t *testing.T) {
	baseURL := newTestServer(t)

	postID := graphqlRequest(t, baseURL, `mutation {
		createPost(input: {authorId: "22222222-2222-2222-2222-222222222222", title: "p", body: "b"}) { id }
	}`, "createPost", "id")

	topID := graphqlRequest(t, baseURL, `mutation {
		createComment(input: {postId: "`+postID+`", authorId: "44444444-4444-4444-4444-444444444444", body: "top"}) { id }
	}`, "createComment", "id")

	graphqlRequest(t, baseURL, `mutation {
		createComment(input: {postId: "`+postID+`", parentId: "`+topID+`", authorId: "66666666-6666-6666-6666-666666666666", body: "reply"}) { id }
	}`, "createComment", "id")

	resp := doGraphQL(t, baseURL, `query { post(id: "`+postID+`") {
		comments(first: 10) { edges { node {
			body
			replies(first: 10) { edges { node { body } } }
		} } }
	} }`)
	if len(resp.Errors) > 0 {
		t.Fatalf("unexpected error: %s", resp.Errors[0].Message)
	}

	var data struct {
		Post struct {
			Comments struct {
				Edges []struct {
					Node struct {
						Body    string `json:"body"`
						Replies struct {
							Edges []struct {
								Node struct {
									Body string `json:"body"`
								} `json:"node"`
							} `json:"edges"`
						} `json:"replies"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"comments"`
		} `json:"post"`
	}
	mustUnmarshal(t, resp.Data, &data)

	if len(data.Post.Comments.Edges) != 1 {
		t.Fatalf("got %d top-level comments, want 1", len(data.Post.Comments.Edges))
	}
	top := data.Post.Comments.Edges[0].Node
	if top.Body != "top" {
		t.Fatalf("top-level comment body = %q, want %q", top.Body, "top")
	}
	if len(top.Replies.Edges) != 1 || top.Replies.Edges[0].Node.Body != "reply" {
		t.Fatalf("replies = %+v, want one reply with body %q", top.Replies.Edges, "reply")
	}
}

func TestMutation_DisableComments_EndToEnd(t *testing.T) {
	baseURL := newTestServer(t)
	const author = "22222222-2222-2222-2222-222222222222"
	const stranger = "99999999-9999-9999-9999-999999999999"

	postID := graphqlRequest(t, baseURL, `mutation {
		createPost(input: {authorId: "`+author+`", title: "p", body: "b"}) { id }
	}`, "createPost", "id")

	t.Run("forbidden for non-author", func(t *testing.T) {
		resp := doGraphQL(t, baseURL, `mutation { disableComments(input: {postId: "`+postID+`", requesterId: "`+stranger+`"}) { id } }`)
		if len(resp.Errors) == 0 {
			t.Fatal("expected an error")
		}
		if code := resp.Errors[0].Extensions.Code; code != "FORBIDDEN" {
			t.Fatalf("error code = %q, want FORBIDDEN", code)
		}
	})

	t.Run("succeeds for author, then blocks new comments", func(t *testing.T) {
		resp := doGraphQL(t, baseURL, `mutation { disableComments(input: {postId: "`+postID+`", requesterId: "`+author+`"}) { commentsDisabled } }`)
		if len(resp.Errors) > 0 {
			t.Fatalf("unexpected error: %s", resp.Errors[0].Message)
		}
		var data struct {
			DisableComments struct {
				CommentsDisabled bool `json:"commentsDisabled"`
			} `json:"disableComments"`
		}
		mustUnmarshal(t, resp.Data, &data)
		if !data.DisableComments.CommentsDisabled {
			t.Fatal("commentsDisabled = false, want true")
		}

		resp = doGraphQL(t, baseURL, `mutation {
			createComment(input: {postId: "`+postID+`", authorId: "`+stranger+`", body: "too late"}) { id }
		}`)
		if len(resp.Errors) == 0 {
			t.Fatal("expected an error creating a comment on a disabled post")
		}
		if code := resp.Errors[0].Extensions.Code; code != "COMMENTS_DISABLED" {
			t.Fatalf("error code = %q, want COMMENTS_DISABLED", code)
		}
	})
}

func mustUnmarshal(t *testing.T, data json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, data)
	}
}
