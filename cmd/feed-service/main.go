package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/zmskv/feed-service/internal/config"
	"github.com/zmskv/feed-service/internal/di"
	"github.com/zmskv/feed-service/internal/presentation/graphql"
	"github.com/zmskv/feed-service/internal/presentation/graphql/generated"
	"github.com/zmskv/feed-service/logger"
)

func main() {
	log := logger.New()
	defer log.Sync()

	cfg := config.Load()

	container, err := di.Build(cfg)
	if err != nil {
		log.Fatal("di build error", zap.Error(err))
	}
	defer container.Close()

	resolver := graphql.NewResolver(container.PostService, container.CommentService)
	schema := generated.NewExecutableSchema(generated.Config{Resolvers: resolver})
	gqlSrv := handler.NewDefaultServer(schema)

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/query", gin.WrapH(gqlSrv))
	r.GET("/playground", gin.WrapH(playground.Handler("GraphQL", "/query")))

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("listening", zap.String("addr", cfg.Addr), zap.String("storage", cfg.Storage))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	stop()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("shutdown error", zap.Error(err))
	}
	log.Info("stopped")
}
