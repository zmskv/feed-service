package graphql_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/zmskv/feed-service/internal/config"
	"github.com/zmskv/feed-service/internal/di"
	"github.com/zmskv/feed-service/internal/presentation/graphql"
)

type gqlError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

func newTestServer(t *testing.T) string {
	t.Helper()
	container, err := di.Build(config.Config{Storage: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { container.Close() })

	srv := httptest.NewServer(graphql.NewRouter(container.PostService, container.CommentService, container.Broadcaster, zap.NewNop()))
	t.Cleanup(srv.Close)
	return srv.URL
}

func doGraphQL(t *testing.T, baseURL, query string) gqlResponse {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"query": query})
	resp, err := http.Post(baseURL+"/query", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("POST /query: %v", err)
	}
	defer resp.Body.Close()

	var parsed gqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return parsed
}

func graphqlRequest(t *testing.T, baseURL, query, operationField, field string) string {
	t.Helper()
	resp := doGraphQL(t, baseURL, query)
	if len(resp.Errors) > 0 {
		t.Fatalf("graphql error: %s", resp.Errors[0].Message)
	}
	var data map[string]map[string]string
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v (%s)", err, resp.Data)
	}
	return data[operationField][field]
}
