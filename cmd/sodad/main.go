package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/builtingit"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/daemon"
	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/osupdate"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/LevitateOS/soda-os/internal/telemetry"
	"github.com/LevitateOS/soda-os/internal/toolchain"
)

func main() {
	// start a logger service
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("sodad stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// set up signal handling for graceful shutdown
	runContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Fetch environment variables or use defaults
	socket := env("SODA_SOCKET", config.DefaultDaemonSocket)
	database := env("SODA_DATABASE", filepath.Join(config.DefaultStateDir, "soda.db"))
	projects := env("SODA_PROJECTS_ROOT", config.DefaultProjectsDir)
	toolchains := env("SODA_TOOLCHAINS_ROOT", config.DefaultToolchainsDir)

	// start a database service
	persistence, err := store.Open(database)
	if err != nil {
		return err
	}

	// If there are jobs that were interrupted by the previous run of sodad, fail them
	// this prevents zombie jobs
	interrupted, err := persistence.FailInterruptedProvisioning(runContext)
	if err != nil {
		return err
	}
	if interrupted != 0 {
		logger.Warn("marked interrupted provisioning jobs failed", slog.Int64("jobs", interrupted))
	}

	// Initialize a new telemetry manager
	// The telemetry manager is responsible for collecting host status information and
	// sending it to the UI.
	observer, err := telemetry.NewManager(telemetry.NewSystemHostSampler(nil, nil))
	if err != nil {
		return err
	}

	// Create an osupdate service to handle system updates
	updates, err := osupdate.New(osupdate.Options{})
	if err != nil {
		return err
	}

	// Creates the daemon service
	service := daemon.New(daemon.Options{
		Store:        persistence,
		Host:         host.New(projects),
		Toolchains:   toolchain.New(toolchains),
		Telemetry:    daemon.NewTelemetryAdapter(observer),
		OSUpdates:    updates,
		BuiltInGit:   builtingit.New(),
		ProjectsRoot: projects,
		Logger:       logger,
	})
	defer service.Close()

	// TODO remove?
	// is this just for display in the UI?
	// is this the result of unwanted abstraction?
	if err = service.ReconcileAllAccess(runContext); err != nil {
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
