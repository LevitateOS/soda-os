package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	helper := projects.NewSystemHelper(linuxhost.NewNative())
	err := execute(ctx, os.Args[1:], os.Stdin, os.Stdout, func(ctx context.Context, action string, input io.Reader) (any, error) {
		actor, err := linuxhost.PKExecCaller()
		if err != nil {
			return nil, err
		}
		return helper.Execute(ctx, actor, action, input)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "soda-workspace-helper:", err)
		if errors.Is(err, errUsage) {
			return 2
		}
		return 1
	}
	return 0
}
