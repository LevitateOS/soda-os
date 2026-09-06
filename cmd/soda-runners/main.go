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
	"github.com/LevitateOS/soda-os/internal/runners"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// A signal must also unblock a request decoder waiting on stdin.
	stopInput := context.AfterFunc(ctx, func() { _ = os.Stdin.Close() })
	defer stopInput()
	coordinator := runners.Coordinator{
		Authorizer: runners.LinuxAuthorizer{Accounts: linuxhost.NewNative()},
		Local:      runners.PKExecInvoker{},
		Privileged: runners.PKExecInvoker{},
	}
	err := execute(ctx, os.Args[1:], os.Stdin, os.Stdout, func(ctx context.Context, action string, input io.Reader) (any, error) {
		current, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("resolve current Linux account: %w", err)
		}
		actor := linuxhost.PKExecIdentity{Username: current.Username, UID: os.Getuid()}
		return coordinator.Execute(ctx, actor, action, input)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "soda-runners:", err)
		if errors.Is(err, errUsage) {
			return 2
		}
		return 1
	}
	return 0
}
