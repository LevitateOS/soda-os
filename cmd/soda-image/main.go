package main

import (
	"context"
	"fmt"
	"os"

	"github.com/LevitateOS/soda-os/internal/image"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "soda-image",
		Short:         "Build Soda OS bootc OCI artifacts",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	var specPath string
	root.PersistentFlags().StringVar(&specPath, "spec", "distro/soda.toml", "path to the Soda distribution specification")
	builder := func() (*image.Builder, error) {
		return image.NewBuilderFromWorkingDirectory(specPath, image.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr})
	}
	root.AddCommand(
		command("check", "validate the pinned Fedora bootc image contract", builder, func(ctx context.Context, b *image.Builder) error { return b.Check(ctx) }),
		command("rpm", "build the three locked Soda RPM inputs", builder, func(ctx context.Context, b *image.Builder) error { return b.BuildRPMs(ctx) }),
		command("oci", "build the Soda bootc OCI archive without loading or publishing it", builder, func(ctx context.Context, b *image.Builder) error { return b.BuildImage(ctx) }),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "soda-image:", err)
		os.Exit(1)
	}
}

func command(name, short string, builder func() (*image.Builder, error), run func(context.Context, *image.Builder) error) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			b, err := builder()
			if err != nil {
				return err
			}
			return run(command.Context(), b)
		},
	}
}
