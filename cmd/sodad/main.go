package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/daemon"
	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/osupdate"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/LevitateOS/soda-os/internal/telemetry"
	"github.com/LevitateOS/soda-os/internal/toolchain"
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
	database := env("SODA_DATABASE", filepath.Join(config.DefaultStateDir, "soda.db"))
	projects := env("SODA_PROJECTS_ROOT", config.DefaultProjectsDir)
	toolchains := env("SODA_TOOLCHAINS_ROOT", config.DefaultToolchainsDir)
	persistence, err := store.Open(database)
	if err != nil {
		return err
	}
	interrupted, err := persistence.FailInterruptedProvisioning(runContext)
	if err != nil {
		return err
	}
	if interrupted != 0 {
		logger.Warn("marked interrupted provisioning jobs failed", slog.Int64("jobs", interrupted))
	}
	system := host.New(projects)
	observer, err := telemetry.NewManager(telemetry.NewSystemHostSampler(nil, nil))
	if err != nil {
		return err
	}
	telemetryAdapter := daemon.NewTelemetryAdapter(observer)
	updates, err := osupdate.New(osupdate.Options{})
	if err != nil {
		return err
	}
	service := daemon.New(daemon.Options{Store: persistence, Host: system, Toolchains: toolchain.New(toolchains), Telemetry: telemetryAdapter, OSUpdates: updates, ProjectsRoot: projects, Logger: logger})
	defer service.Close()
	if err = service.ReconcileAllAuthorizedKeys(runContext); err != nil {
		return err
	}
	server, err := daemon.ListenUnix(socket, service, logger)
	if err != nil {
		return err
	}
	defer server.Stop()
	observer.Run(runContext)
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
