package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/issues/internal/api"
	"github.com/r59q/hub-rearranger/services/issues/internal/domain"
	githubcatalog "github.com/r59q/hub-rearranger/services/issues/internal/infrastructure/github"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("issues service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return errors.New("GITHUB_TOKEN is required")
	}
	address := envOrDefault("ISSUES_ADDR", "127.0.0.1:8081")
	githubClient := gh.NewClient(&http.Client{Timeout: 20 * time.Second}).WithAuthToken(token)
	catalog := githubcatalog.NewCatalog(githubClient)
	service := domain.NewIssueService(catalog)

	server := &http.Server{
		Addr: address, Handler: api.NewHandler(service, logger), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}()

	logger.Info("issues service listening", "address", address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
