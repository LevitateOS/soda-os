package main

import (
	"context"
	"io"

	"github.com/LevitateOS/soda-os/internal/acceptance"
	"github.com/spf13/cobra"
)

type suiteRunner func(context.Context, acceptance.RunOptions, io.Writer) (acceptance.RunResult, error)

func newCommand(suite suiteRunner) *cobra.Command {
	root := &cobra.Command{
		Use:           "soda-acceptance",
		Short:         "Run and report matching-native Soda OS product acceptance",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = false
	root.AddCommand(runCommand(suite))
	return root
}
