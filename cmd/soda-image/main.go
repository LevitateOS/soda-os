package main

import (
	"context"
	"fmt"
	"os"

	"github.com/LevitateOS/soda-os/internal/image"
	"github.com/LevitateOS/soda-os/internal/installer"
	"github.com/LevitateOS/soda-os/internal/release"
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
		Short: "build an AArch64 installer ISO from one signed exact Soda image",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			imageBuilder, err := builder()
			if err != nil {
				return err
			}
			isoBuilder := installer.NewBuilder(imageBuilder.Root, imageBuilder.Spec, image.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr})
			result, err := isoBuilder.Build(command.Context(), options)
			if err != nil {
				return err
			}
			fmt.Printf("Built installer ISO: %s\nPayload provenance: %s\n", result.ISOPath, result.ProvenancePath)
			return nil
		},
	}
	command.Flags().StringVar(&options.ImageReference, "image", "", "signed exact registry.soda.local/soda/os@sha256 payload")
	command.Flags().StringVar(&options.ArchivePath, "archive", "", "local OCI archive matching the exact payload")
	command.Flags().StringVar(&options.RegistryCA, "registry-ca", "", "PEM CA certificate for registry.soda.local")
	command.Flags().StringVar(&options.PublicKey, "public-key", "", "Soda Cosign public key")
	command.Flags().StringVar(&options.CosignPath, "cosign", ".artifacts/tools/cosign", "pinned Cosign executable")
	command.Flags().StringVar(&options.ToolLock, "tool-lock", "packaging/installer/image-builder.lock", "pinned Image Builder tool contract")
	command.Flags().StringVar(&options.OutputDir, "output-dir", ".artifacts/images", "installer artifact directory")
	for _, name := range []string{"image", "archive", "registry-ca", "public-key"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func releaseCommand(builder func() (*image.Builder, error)) *cobra.Command {
	var options release.Options
	command := &cobra.Command{
		Use:   "publish",
		Short: "publish and sign one exact Soda bootc release",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			b, err := builder()
			if err != nil {
				return err
			}
			publisher, err := release.NewPublisher(b.Spec, options, image.OSRunner{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
			if err != nil {
				return err
			}
			result, err := publisher.Publish(command.Context(), options)
			if err != nil {
				return err
			}
			if result.RecordPath == "" {
				fmt.Printf("Prepared signed image %s; no release record or current tag was written\n", result.ImageReference)
				return nil
			}
			fmt.Printf("Published %s\nRelease record: %s\nSignature bundle: %s\n", result.ImageReference, result.RecordPath, result.BundlePath)
			return nil
		},
	}
	command.Flags().StringVar(&options.ArchivePath, "archive", "", "path to the AArch64 Soda OCI archive")
	command.Flags().StringVar(&options.RegistryCA, "registry-ca", "", "PEM CA certificate for registry.soda.local")
	command.Flags().StringVar(&options.PublicKey, "public-key", "", "Soda Cosign public key")
	command.Flags().StringVar(&options.PrivateKey, "signing-key", "", "passphrase-protected Soda Cosign private key")
	command.Flags().StringVar(&options.ISOPath, "iso", "", "optional installer ISO to bind into the release record")
	command.Flags().BoolVar(&options.DeferCurrent, "defer-current", false, "sign and verify the exact image for ISO construction without writing a record or current tag")
	command.Flags().StringVar(&options.OutputDir, "output-dir", ".artifacts/releases", "signed release record directory")
	command.Flags().StringVar(&options.CosignPath, "cosign", ".artifacts/tools/cosign", "pinned Cosign executable")
	command.Flags().StringVar(&options.ToolLock, "tool-lock", "packaging/release/tools.lock", "pinned release tool checksums")
	command.Flags().StringVar(&options.InstallerArchive, "installer-archive", ".artifacts/installer/soda-installer-environment.oci.tar", "build-only installer environment used to inspect --iso")
	command.Flags().StringVar(&options.InstallerToolLock, "installer-tool-lock", "packaging/installer/image-builder.lock", "pinned Image Builder contract used to inspect --iso")
	for _, name := range []string{"archive", "registry-ca", "public-key", "signing-key"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
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
