package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/acceptance"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	command := newCommand(acceptance.Run)
	command.SetArgs(os.Args[1:])
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	if err := command.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "soda-acceptance:", err)
		return 1
	}
	return 0
}
