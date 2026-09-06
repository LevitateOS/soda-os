package main

import (
	"context"
	"errors"
	"io"

	"github.com/LevitateOS/soda-os/internal/updates"
	"github.com/spf13/cobra"
)

type updateOperations interface {
	Status(context.Context) (updates.Host, error)
	Check(context.Context) (updates.Host, error)
	Update(context.Context) error
}

type updateFactory func(stdout, stderr io.Writer) updateOperations

func newCommand(architecture string, euid int, connect updateFactory) *cobra.Command {
	root := &cobra.Command{
		Use:           "soda-updates",
		Short:         "Check and update the native bootc image source",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			if euid != 0 {
				return errors.New("enable Cockpit administrative access to manage Soda Updates")
			}
			if architecture == "" {
				return errors.New("Soda Updates requires x86_64 or aarch64")
			}
			return nil
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(statusCommand(connect), checkCommand(connect), updateCommand(connect))
	return root
}
