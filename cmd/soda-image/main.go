package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LevitateOS/soda-os/internal/build/image"
	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "soda-image",
		Short:         "Build Soda OS bootc OCI artifacts",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	var specPath, architecture string
	root.PersistentFlags().StringVar(&specPath, "spec", "distro/soda.toml", "path to the Soda distribution specification")
	root.PersistentFlags().StringVar(&architecture, "architecture", "", "Soda architecture to operate on: aarch64 or x86_64")
	_ = root.MarkPersistentFlagRequired("architecture")
	builder := func() (*image.Builder, error) {
		return image.NewBuilderFromWorkingDirectory(specPath, architecture, process.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr})
	}
	root.AddCommand(
		command("check", "validate the pinned Fedora bootc image contract", builder, func(ctx context.Context, b *image.Builder) error { return b.Check(ctx) }),
		command("rpm", "build the four locked Soda RPM inputs", builder, func(ctx context.Context, b *image.Builder) error { return b.BuildRPMs(ctx) }),
	)
	oci := command("oci", "build the Soda bootc OCI archive without loading or publishing it", builder, func(ctx context.Context, b *image.Builder) error {
		return b.BuildImage(ctx)
	})
	root.AddCommand(oci)
	root.AddCommand(releaseCommand(builder))
	root.AddCommand(installerCommand(builder))
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "soda-image:", err)
		os.Exit(1)
	}
}

func installerCommand(builder func() (*image.Builder, error)) *cobra.Command {
	var options installer.Options
	command := &cobra.Command{
		Use:   "iso",
		Short: "build a platform-matched installer ISO from a local Soda OCI archive",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			imageBuilder, err := builder()
			if err != nil {
				return err
			}
			if options.ToolLock == "" {
				options.ToolLock = imageBuilder.Spec.Platform.Installer.ToolLock
			}
			isoBuilder := installer.NewBuilder(imageBuilder.Root, imageBuilder.Spec, process.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr})
			isoPath, err := isoBuilder.Build(command.Context(), options)
			if err != nil {
				return err
			}
			fmt.Printf("Built installer ISO: %s\nChecksum: %s.sha256\n", isoPath, isoPath)
			return nil
		},
	}
	command.Flags().StringVar(&options.ArchivePath, "archive", "", "local single-platform Soda OCI archive")
	command.Flags().StringVar(&options.ToolLock, "tool-lock", "", "pinned Image Builder tool contract (defaults to the selected platform lock)")
	command.Flags().StringVar(&options.OutputDir, "output-dir", ".artifacts/images", "installer artifact directory")
	_ = command.MarkFlagRequired("archive")
	return command
}

type releaseCommandState struct {
	builder func() (*image.Builder, error)
	record  release.RecordOptions
}

func releaseCommand(builder func() (*image.Builder, error)) *cobra.Command {
	state := &releaseCommandState{builder: builder}
	command := &cobra.Command{
		Use:   "record",
		Short: "inspect a local OCI archive and installer ISO and write release metadata",
		Args:  cobra.NoArgs,
		RunE:  state.run,
	}
	command.Flags().StringVar(&state.record.ArchivePath, "archive", "", "path to the selected-architecture Soda OCI archive")
	command.Flags().StringVar(&state.record.ISOPath, "iso", "", "installer ISO built from the local OCI archive")
	command.Flags().StringVar(&state.record.OutputDir, "output-dir", ".artifacts/releases", "release record directory")
	command.Flags().StringVar(&state.record.InstallerArchive, "installer-archive", "", "build-only installer environment used to inspect --iso (defaults to the selected architecture artifact)")
	command.Flags().StringVar(&state.record.InstallerToolLock, "installer-tool-lock", "", "pinned Image Builder contract used to inspect --iso (defaults to the selected platform lock)")
	for _, name := range []string{"archive", "iso"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func (state *releaseCommandState) run(command *cobra.Command, _ []string) error {
	builder, err := state.builder()
	if err != nil {
		return err
	}
	if state.record.InstallerArchive == "" {
		state.record.InstallerArchive = filepath.Join(".artifacts", "installer", "soda-installer-environment-"+builder.Spec.Platform.Architecture.Artifact+".oci.tar")
	}
	if state.record.InstallerToolLock == "" {
		state.record.InstallerToolLock = builder.Spec.Platform.Installer.ToolLock
	}
	publisher, err := release.NewPublisher(builder.Root, builder.Spec, process.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr})
	if err != nil {
		return err
	}
	result, err := publisher.CreateRecord(command.Context(), state.record)
	if err != nil {
		return err
	}
	fmt.Printf("Recorded %s\nRelease record: %s\n", result.ImageReference, result.RecordPath)
	return nil
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
