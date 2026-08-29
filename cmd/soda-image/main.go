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
		command("rpm", "build the three locked Soda RPM inputs", builder, func(ctx context.Context, b *image.Builder) error { return b.BuildRPMs(ctx) }),
	)
	var registryCA, publicKey string
	oci := command("oci", "build the Soda bootc OCI archive without loading or publishing it", builder, func(ctx context.Context, b *image.Builder) error {
		b.RegistryCA = registryCA
		b.SigningPublicKey = publicKey
		return b.BuildImage(ctx)
	})
	oci.Flags().StringVar(&registryCA, "registry-ca", "", "PEM CA certificate embedded for registry.soda.local")
	oci.Flags().StringVar(&publicKey, "public-key", "", "Soda Cosign public key embedded for update verification")
	_ = oci.MarkFlagRequired("registry-ca")
	_ = oci.MarkFlagRequired("public-key")
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
		Short: "build a platform-matched installer ISO from one signed exact Soda image",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			imageBuilder, err := builder()
			if err != nil {
				return err
			}
			if options.ToolLock == "" {
				options.ToolLock = imageBuilder.Spec.Platform.InstallerToolLock
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
	command.Flags().StringVar(&options.ImageReference, "image", "", "signed exact registry.soda.local/soda/os@sha256 payload")
	command.Flags().StringVar(&options.ArchivePath, "archive", "", "local OCI archive matching the exact payload")
	command.Flags().StringVar(&options.RegistryCA, "registry-ca", "", "PEM CA certificate for registry.soda.local")
	command.Flags().StringVar(&options.PublicKey, "public-key", "", "Soda Cosign public key")
	command.Flags().StringVar(&options.CosignPath, "cosign", ".artifacts/tools/cosign", "pinned Cosign executable")
	command.Flags().StringVar(&options.ToolLock, "tool-lock", "", "pinned Image Builder tool contract (defaults to the selected platform lock)")
	command.Flags().StringVar(&options.OutputDir, "output-dir", ".artifacts/images", "installer artifact directory")
	for _, name := range []string{"image", "archive", "registry-ca", "public-key"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

type releaseCommandState struct {
	builder      func() (*image.Builder, error)
	signing      release.SigningOptions
	publication  release.PublicationOptions
	deferCurrent bool
}

func releaseCommand(builder func() (*image.Builder, error)) *cobra.Command {
	state := &releaseCommandState{builder: builder}
	command := &cobra.Command{
		Use:   "publish",
		Short: "publish and sign one exact Soda bootc release",
		Args:  cobra.NoArgs,
		RunE:  state.run,
	}
	command.Flags().StringVar(&state.publication.ArchivePath, "archive", "", "path to the selected-architecture Soda OCI archive")
	command.Flags().StringVar(&state.signing.RegistryCA, "registry-ca", "", "PEM CA certificate for registry.soda.local")
	command.Flags().StringVar(&state.signing.PublicKey, "public-key", "", "Soda Cosign public key")
	command.Flags().StringVar(&state.signing.PrivateKey, "signing-key", "", "passphrase-protected Soda Cosign private key")
	command.Flags().StringVar(&state.publication.ISOPath, "iso", "", "optional installer ISO to bind into the release record")
	command.Flags().BoolVar(&state.deferCurrent, "defer-current", false, "sign and verify the exact image for ISO construction without writing a record or current tag")
	command.Flags().StringVar(&state.publication.OutputDir, "output-dir", ".artifacts/releases", "signed release record directory")
	command.Flags().StringVar(&state.signing.CosignPath, "cosign", ".artifacts/tools/cosign", "pinned Cosign executable")
	command.Flags().StringVar(&state.signing.ToolLock, "tool-lock", "distro/locks/release-tools.toml", "pinned release tool checksums")
	command.Flags().StringVar(&state.publication.InstallerArchive, "installer-archive", "", "build-only installer environment used to inspect --iso (defaults to the selected architecture artifact)")
	command.Flags().StringVar(&state.publication.InstallerToolLock, "installer-tool-lock", "", "pinned Image Builder contract used to inspect --iso (defaults to the selected platform lock)")
	for _, name := range []string{"archive", "registry-ca", "public-key", "signing-key"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func (state *releaseCommandState) run(command *cobra.Command, _ []string) error {
	builder, err := state.builder()
	if err != nil {
		return err
	}
	if state.publication.InstallerArchive == "" {
		state.publication.InstallerArchive = filepath.Join(".artifacts", "installer", "soda-installer-environment-"+builder.Spec.Platform.ArtifactArchitecture+".oci.tar")
	}
	if state.publication.InstallerToolLock == "" {
		state.publication.InstallerToolLock = builder.Spec.Platform.InstallerToolLock
	}
	publisher, err := release.NewPublisher(builder.Root, builder.Spec, state.signing, process.OSRunner{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
	if err != nil {
		return err
	}
	if state.deferCurrent {
		reference, prepareErr := publisher.Prepare(command.Context(), state.publication.ArchivePath)
		if prepareErr == nil {
			fmt.Printf("Prepared signed image %s; no release record or current tag was written\n", reference)
		}
		return prepareErr
	}
	result, err := publisher.Publish(command.Context(), state.publication)
	if err != nil {
		return err
	}
	fmt.Printf("Published %s\nRelease record: %s\nSignature bundle: %s\n", result.ImageReference, result.RecordPath, result.BundlePath)
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
