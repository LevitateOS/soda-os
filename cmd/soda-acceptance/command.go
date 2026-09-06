package main

import (
	"context"
	"io"

	"github.com/LevitateOS/soda-os/internal/acceptance"
	"github.com/spf13/cobra"
)

type suiteRunner func(context.Context, acceptance.RunOptions, io.Writer) (acceptance.RunResult, error)

type recordSigner func(context.Context, string, acceptance.RecordOptions, io.Writer, io.Writer) (acceptance.RecordResult, error)

type recordVerifier func(context.Context, string, string, io.Writer, io.Writer) error

func newCommand(suite suiteRunner, sign recordSigner, verify recordVerifier) *cobra.Command {
	root := &cobra.Command{
		Use:           "soda-acceptance",
		Short:         "Run and record matching-native Soda OS product acceptance",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = false
	root.AddCommand(runCommand(suite), recordCommand(sign), verifyCommand(verify))
	return root
}
