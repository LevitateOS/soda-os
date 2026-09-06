package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	command := newCommand(func(specPath string, stdout, stderr io.Writer) (publicationOperations, error) {
		root, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		return publication(root, specPath, process.OSRunner{Stdout: stdout, Stderr: stderr})
	})
	command.SetArgs(os.Args[1:])
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	if err := command.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "soda-release:", err)
		return 1
	}
	return 0
}

func publication(root, specPath string, runner process.Runner) (*release.Publication, error) {
	aarch64, err := config.LoadDistro(specPath, "aarch64")
	if err != nil {
		return nil, err
	}
	x86, err := config.LoadDistro(specPath, "x86_64")
	if err != nil {
		return nil, err
	}
	return release.NewPublication(root, aarch64, x86, runner)
}
