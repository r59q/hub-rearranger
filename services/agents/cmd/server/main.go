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

	"github.com/r59q/hub-rearranger/services/agents/internal/api"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/config"
	githubinfra "github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("agents service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	validator, err := profiles.NewValidator()
	if err != nil {
		return errors.New("profile schema could not be initialized")
	}
	client := gh.NewClient(&http.Client{Timeout: 8 * time.Second})
	if token := config.GitHubToken(); token != "" {
		client = client.WithAuthToken(token)
	}
	service := domain.NewService(domain.NewProfileService(githubinfra.NewProfileReader(client), validator))
	server := &http.Server{
		Addr: config.Address(), Handler: api.NewHandler(service, logger),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownDone := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownDone <- server.Shutdown(shutdownCtx)
	}()

	logger.Info("agents service listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	// ListenAndServe returns as soon as listeners close. Wait for in-flight
	// requests to finish before allowing the process to exit.
	return <-shutdownDone
}
