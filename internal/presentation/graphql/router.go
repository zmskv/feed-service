package graphql

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"

	"github.com/zmskv/feed-service/internal/presentation/graphql/generated"
)

func NewRouter(posts PostService, comments CommentService, sub Subscriber) *gin.Engine {
	resolver := NewResolver(posts, comments, sub)
	schema := generated.NewExecutableSchema(generated.Config{Resolvers: resolver})
	gqlSrv := handler.NewDefaultServer(schema)
	gqlSrv.SetErrorPresenter(ErrorPresenter)

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
