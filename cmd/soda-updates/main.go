// soda-updates is the synchronous administrator-only Cockpit Updates command.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/LevitateOS/soda-os/internal/updates"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	architecture := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	command := newCommand(architecture, os.Geteuid(), func(stdout, stderr io.Writer) updateOperations {
		return nativeUpdates{
			operations: updates.Operations{
				Queries: process.OSRunner{Stderr: stderr},
				Runner:  process.OSRunner{Stdout: stdout, Stderr: stderr},
			},
			lock: func() (io.Closer, error) { return updates.Lock("/run/soda-updates.lock") },
		}
	})
	command.SetArgs(os.Args[1:])
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	if err := command.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "soda-updates:", err)
		return 1
	}
	return 0
}
