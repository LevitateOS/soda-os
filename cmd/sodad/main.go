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
	"github.com/LevitateOS/soda-os/internal/observe"
	"github.com/LevitateOS/soda-os/internal/store"
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
	for _, path := range []string{filepath.Dir(database), projects, toolchains} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
		if err := os.Chmod(path, 0o755); err != nil {
			return err
		}
	}
	persistence, err := store.Open(database)
	if err != nil {
		return err
	}
	system := host.New(projects, true)
	observer, err := observe.NewManager(observe.Dependencies{Store: persistence, Host: observe.NewSystemHostSampler(nil, nil), Git: observe.NewSystemGitInspector(nil), Sessions: observe.NewSystemSessionInspector(nil)})
	if err != nil {
		return err
	}
	defer observer.Broker().Close()
	observability := daemon.NewObservability(observer)
	service := daemon.New(daemon.Options{Store: persistence, Host: system, Toolchains: toolchain.New(toolchains), Telemetry: observability, Events: observability, EventSource: observability, ProjectsRoot: projects, Logger: logger})
	defer service.Close()
	if err = service.BootstrapInstallerAdministrator(runContext); err != nil {
		return err
	}
	server, err := daemon.ListenUnix(socket, "soda-api", service, logger)
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
