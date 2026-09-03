package main

import (
	"fmt"
	"os"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/spf13/cobra"
)

func main() {
	command := newCommand(process.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr})
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "soda-release:", err)
		os.Exit(1)
	}
}

func newCommand(runner process.Runner) *cobra.Command {
	var specPath string
	command := &cobra.Command{
		Use:           "soda-release",
		Short:         "operate one append-only Soda OS GitHub release",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}
	command.PersistentFlags().StringVar(&specPath, "spec", "distro/soda.toml", "path to the Soda distribution specification")
	command.AddCommand(imageStageCommand(&specPath, runner), imagePromoteCommand(&specPath, runner), draftCommand(&specPath, runner), uploadCommand(&specPath, runner), publishCommand(&specPath, runner))
	return command
}

func imageStageCommand(specPath *string, runner process.Runner) *cobra.Command {
	var options release.ImageStageOptions
	command := &cobra.Command{
		Use:   "image-stage",
		Short: "publish and verify one immutable matching-native GHCR candidate image",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := publication(*specPath, runner)
			if err != nil {
				return err
			}
			result, err := publication.ImageStage(command.Context(), options)
			if err != nil {
				return err
			}
			fmt.Printf("Published %s candidate: %s\n", result.Architecture, result.Reference)
			return nil
		},
	}
	command.Flags().StringVar(&options.Architecture, "architecture", "", "matching-native Soda architecture")
	command.Flags().StringVar(&options.ArchivePath, "archive", "", "matching-native local Soda OCI archive")
	_ = command.MarkFlagRequired("architecture")
	_ = command.MarkFlagRequired("archive")
	return command
}

func imagePromoteCommand(specPath *string, runner process.Runner) *cobra.Command {
	var options release.ImagePromoteOptions
	command := &cobra.Command{
		Use:   "image-promote",
		Short: "promote one verified immutable candidate to its versioned GHCR tag",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := publication(*specPath, runner)
			if err != nil {
				return err
			}
			result, err := publication.ImagePromote(command.Context(), options)
			if err != nil {
				return err
			}
			fmt.Printf("Promoted %s image: %s\n", result.Architecture, result.Reference)
			return nil
		},
	}
	command.Flags().StringVar(&options.Architecture, "architecture", "", "matching-native Soda architecture")
	command.Flags().StringVar(&options.RecordPath, "record", "", "matching-native schema-3 Soda release record")
	_ = command.MarkFlagRequired("architecture")
	_ = command.MarkFlagRequired("record")
	return command
}

func draftCommand(specPath *string, runner process.Runner) *cobra.Command {
	var options release.DraftOptions
	command := &cobra.Command{
		Use:   "draft",
		Short: "create an empty GitHub draft for the clean Soda source revision",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := publication(*specPath, runner)
			if err != nil {
				return err
			}
			result, err := publication.Draft(command.Context(), options)
			if err != nil {
				return err
			}
			fmt.Printf("Created GitHub draft %s for %s\n", result.Tag, result.Revision)
			return nil
		},
	}
	command.Flags().StringVar(&options.NotesPath, "notes-file", "", "regular file containing the release notes")
	_ = command.MarkFlagRequired("notes-file")
	return command
}

func uploadCommand(specPath *string, runner process.Runner) *cobra.Command {
	var options release.UploadOptions
	command := &cobra.Command{
		Use:   "upload",
		Short: "validate and upload one matching-native installer asset set",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := publication(*specPath, runner)
			if err != nil {
				return err
			}
			result, err := publication.Upload(command.Context(), options)
			if err != nil {
				return err
			}
			fmt.Printf("Uploaded %s assets to GitHub draft %s\n", options.Architecture, result.Tag)
			return nil
		},
	}
	command.Flags().StringVar(&options.Architecture, "architecture", "", "matching-native Soda architecture (aarch64 or x86_64)")
	command.Flags().StringVar(&options.ISOPath, "iso", "", "matching-native Soda installer ISO")
	command.Flags().StringVar(&options.QCOW2ZSTPath, "qcow2-zst", "", "matching-native compressed Soda QCOW2 download")
	command.Flags().StringVar(&options.RecordPath, "record", "", "matching-native Soda release record")
	command.Flags().StringVar(&options.RecordBundlePath, "record-bundle", "", "keyless-signed bundle for the matching-native release record")
	for _, name := range []string{"architecture", "iso", "qcow2-zst", "record", "record-bundle"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func publishCommand(specPath *string, runner process.Runner) *cobra.Command {
	return &cobra.Command{
		Use:   "publish",
		Short: "publish a complete validated Soda GitHub draft",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := publication(*specPath, runner)
			if err != nil {
				return err
			}
			result, err := publication.Publish(command.Context())
			if err != nil {
				return err
			}
			fmt.Printf("Published GitHub release %s for %s\n", result.Tag, result.Revision)
			return nil
		},
	}
}

func publication(specPath string, runner process.Runner) (*release.Publication, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	aarch64, err := config.LoadDistro(specPath, "aarch64")
	if err != nil {
		return nil, err
	}
	x86, err := config.LoadDistro(specPath, "x86_64")
	if err != nil {
		return nil, err
	}
	return release.NewPublication(root, aarch64, x86, runner)
}
