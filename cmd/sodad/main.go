package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/daemon"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("sodad stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	runContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	socket := env("SODA_SOCKET", config.DefaultDaemonSocket)
	service := daemon.New()
	server, err := daemon.ListenUnix(socket, service, logger)
	if err != nil {
		return err
	}
	defer server.Stop()
	serveError := make(chan error, 1)
	go func() { serveError <- server.Serve() }()
	logger.Info("sodad listening", slog.String("socket", socket))
	select {
	case err = <-serveError:
		return err
	case <-runContext.Done():
		return nil
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
