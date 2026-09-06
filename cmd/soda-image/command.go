package main

import (
	"context"
	"io"

	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/spf13/cobra"
)

type imageOperations interface {
	Check(context.Context) error
	BuildRPMs(context.Context) error
	BuildImage(context.Context, string) (string, error)
	BuildISO(context.Context, installer.Options) (string, error)
	BuildQCOW2(context.Context, installer.QCOW2Options) (installer.QCOW2Result, error)
	CreateRecord(context.Context, release.RecordOptions) (release.Result, error)
}

type imageFactory func(specPath, architecture string, stdout, stderr io.Writer) (imageOperations, error)

func newCommand(connect imageFactory) *cobra.Command {
	var specPath, architecture string
	root := &cobra.Command{
		Use:           "soda-image",
		Short:         "Build Soda OS bootc OCI artifacts",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = false
	flags := root.PersistentFlags()
	flags.StringVar(&specPath, "spec", "distro/soda.toml", "path to the Soda distribution specification")
	flags.StringVar(&architecture, "architecture", "", "Soda architecture to operate on: aarch64 or x86_64")
	_ = root.MarkPersistentFlagRequired("architecture")
	root.AddCommand(checkCommand(&specPath, &architecture, connect), rpmCommand(&specPath, &architecture, connect), ociCommand(&specPath, &architecture, connect))
	root.AddCommand(isoCommand(&specPath, &architecture, connect), qcow2Command(&specPath, &architecture, connect), recordCommand(&specPath, &architecture, connect))
	return root
}
