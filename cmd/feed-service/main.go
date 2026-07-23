package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/zmskv/feed-service/internal/config"
	"github.com/zmskv/feed-service/internal/di"
	"github.com/zmskv/feed-service/internal/presentation/graphql"
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

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: graphql.NewRouter(container.PostService, container.CommentService, container.Broadcaster, log),
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
