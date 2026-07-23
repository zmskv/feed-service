package graphql

import (
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"
	"github.com/vektah/gqlparser/v2/ast"
	"go.uber.org/zap"

	"github.com/zmskv/feed-service/internal/presentation/graphql/generated"
)

func NewRouter(posts PostService, comments CommentService, sub Subscriber, log *zap.Logger) *gin.Engine {
	resolver := NewResolver(posts, comments, sub)
	schema := generated.NewExecutableSchema(generated.Config{Resolvers: resolver})

	gqlSrv := handler.New(schema)
	gqlSrv.AddTransport(transport.Websocket{KeepAlivePingInterval: 10 * time.Second})
	gqlSrv.AddTransport(transport.Options{})
	gqlSrv.AddTransport(transport.GET{})
	gqlSrv.AddTransport(transport.POST{})
	gqlSrv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	gqlSrv.Use(extension.Introspection{})
	gqlSrv.SetErrorPresenter(NewErrorPresenter(log))

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	queryHandler := gin.WrapH(Middleware(comments)(gqlSrv))
	r.POST("/query", queryHandler)
	r.GET("/query", queryHandler) // websocket upgrade for subscriptions arrives as GET
	r.GET("/playground", gin.WrapH(playground.Handler("GraphQL", "/query")))
	return r
}
