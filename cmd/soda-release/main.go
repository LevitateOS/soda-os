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
	command.AddCommand(draftCommand(&specPath, runner), uploadCommand(&specPath, runner), publishCommand(&specPath, runner))
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
	command.Flags().StringVar(&options.RecordPath, "record", "", "matching-native Soda release record")
	for _, name := range []string{"architecture", "iso", "record"} {
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
