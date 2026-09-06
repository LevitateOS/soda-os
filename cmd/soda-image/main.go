package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/build/image"
	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/LevitateOS/soda-os/internal/process"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	command := newCommand(func(specPath, architecture string, stdout, stderr io.Writer) (imageOperations, error) {
		root, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		return nativeBuilder(root, specPath, architecture, process.OSRunner{Stdout: stdout, Stderr: stderr})
	})
	command.SetArgs(os.Args[1:])
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	if err := command.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "soda-image:", err)
		return 1
	}
	return 0
}

func nativeBuilder(root, specPath, architecture string, runner process.Runner) (imageOperations, error) {
	builder, err := image.NewBuilder(root, specPath, architecture, runner)
	if err != nil {
		return nil, err
	}
	return nativeImage{
		Builder:   builder,
		installer: installer.NewBuilder(builder.Root, builder.Spec, runner),
	}, nil
}
