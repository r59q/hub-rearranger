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

	"github.com/r59q/hub-rearranger/services/identity/internal/api"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
	"github.com/r59q/hub-rearranger/services/identity/internal/infrastructure/config"
	githubinfra "github.com/r59q/hub-rearranger/services/identity/internal/infrastructure/github"
	"github.com/r59q/hub-rearranger/services/identity/internal/infrastructure/sqlite"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("identity service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	c, err := config.Load()
	if err != nil {
		return err
	}

	service := domain.NewService(nil, nil, nil)
	if c.Enabled {
		store, err := sqlite.Open(c.DBPath, c.Key)
		if err != nil {
			return errors.New("identity storage could not be initialized")
		}
		defer func() { _ = store.Close() }()

		provider := githubinfra.New(c.AppID, c.ClientID, c.ClientSecret, c.Origin+"/auth/callback", &http.Client{Timeout: 8 * time.Second})
		service = domain.NewService(provider, store, nil)
	}

	server := &http.Server{Addr: c.Address, Handler: api.NewHandler(service, c.Origin, c.Secure), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		done <- server.Shutdown(shutdown)
	}()

	logger.Info("identity service listening", "address", server.Addr, "sign_in_enabled", c.Enabled)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return errors.New("identity HTTP server failed")
	}
	return <-done
}
