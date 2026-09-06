package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"os/user"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	coordinator := projects.NewSystemCoordinator(linuxhost.NewNative())
	err := execute(ctx, os.Args[1:], os.Stdin, os.Stdout, func(ctx context.Context, action string, input io.Reader) (any, error) {
		current, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("resolve current Linux account: %w", err)
		}
		return coordinator.Execute(ctx, current.Username, action, input)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "soda-projects:", err)
		if errors.Is(err, errUsage) {
			return 2
		}
		return 1
	}
	return 0
}
