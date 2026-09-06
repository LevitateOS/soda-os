package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/acceptance"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	command := newCommand(acceptance.Run,
		func(ctx context.Context, specPath string, options acceptance.RecordOptions, stdout, stderr io.Writer) (acceptance.RecordResult, error) {
			return signRecord(ctx, specPath, options, process.OSRunner{Stdout: stdout, Stderr: stderr})
		},
		func(ctx context.Context, path, revision string, stdout, stderr io.Writer) error {
			return acceptance.VerifySignedRecord(ctx, path, revision, process.OSRunner{Stdout: stdout, Stderr: stderr})
		})
	command.SetArgs(os.Args[1:])
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	if err := command.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "soda-acceptance:", err)
		return 1
	}
	return 0
}

func signRecord(ctx context.Context, specPath string, options acceptance.RecordOptions, runner process.Runner) (acceptance.RecordResult, error) {
	var err error
	options.ARM64Spec, err = config.LoadDistro(specPath, "aarch64")
	if err != nil {
		return acceptance.RecordResult{}, err
	}
	options.X86Spec, err = config.LoadDistro(specPath, "x86_64")
	if err != nil {
		return acceptance.RecordResult{}, err
	}
	return acceptance.CreateSignedRecord(ctx, options, runner)
}
