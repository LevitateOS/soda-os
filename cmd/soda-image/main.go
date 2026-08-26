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
		Short:         "Build Soda OS artifacts",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	var specPath string
	root.PersistentFlags().StringVar(&specPath, "spec", "distro/soda.toml", "path to the Soda distribution specification")
	builder := func() (*image.Builder, error) {
		return image.NewBuilderFromWorkingDirectory(specPath, image.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr})
	}
	root.AddCommand(
		command("check", "validate the Soda installer contract", builder, func(ctx context.Context, b *image.Builder) error { return b.Check(ctx) }),
		command("verify", "verify the signed Rocky source media", builder, func(ctx context.Context, b *image.Builder) error { return b.Verify(ctx) }),
		command("rpm", "build Soda RPMs and repository", builder, func(ctx context.Context, b *image.Builder) error { return b.BuildRPMs(ctx) }),
	)
	var automated bool
	iso := command("iso", "build a compact Soda installation ISO", builder, func(ctx context.Context, b *image.Builder) error { return b.BuildISO(ctx, automated) })
	iso.Flags().BoolVar(&automated, "automated", false, "build the automated test installer")
	root.AddCommand(iso)
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
