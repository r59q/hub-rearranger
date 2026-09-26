package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/repositories/internal/api"
	"github.com/r59q/hub-rearranger/services/repositories/internal/domain"
	githubcatalog "github.com/r59q/hub-rearranger/services/repositories/internal/infrastructure/github"
	"github.com/r59q/hub-rearranger/services/repositories/internal/infrastructure/sqlite"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("repositories service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return errors.New("GITHUB_TOKEN is required")
	}
	address := envOrDefault("REPOSITORIES_ADDR", "127.0.0.1:8080")
	databasePath := envOrDefault("REPOSITORIES_DB_PATH", "./data/repositories.db")
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o750); err != nil {
		return errors.New("create database directory: " + err.Error())
	}

	store, err := sqlite.Open(databasePath)
	if err != nil {
		return err
	}
	defer store.Close()

	githubClient := gh.NewClient(&http.Client{Timeout: 20 * time.Second}).WithAuthToken(token)
	catalog := githubcatalog.NewCatalog(githubClient)
	service := domain.NewRepositoryService(catalog, store)

	server := &http.Server{
		Addr:              address,
		Handler:           api.NewHandler(service, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
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

	logger.Info("repositories service listening", "address", address)
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
