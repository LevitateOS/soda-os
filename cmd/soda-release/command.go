package main

import (
	"context"
	"io"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/spf13/cobra"
)

type publicationOperations interface {
	ImageStage(context.Context, release.ImageStageOptions) (release.ImageResult, error)
	ImagePromote(context.Context, release.ImagePromoteOptions) (release.ImageResult, error)
	SignRecord(context.Context, release.RecordSignOptions) (release.RecordSignResult, error)
	Draft(context.Context, release.DraftOptions) (release.PublicationResult, error)
	Upload(context.Context, release.UploadOptions) (release.PublicationResult, error)
	Publish(context.Context, release.PublishOptions) (release.PublicationResult, error)
}

type publicationFactory func(specPath string, stdout, stderr io.Writer) (publicationOperations, error)

func newCommand(connect publicationFactory) *cobra.Command {
	var specPath string
	root := &cobra.Command{
		Use:           "soda-release",
		Short:         "operate one append-only Soda OS GitHub release",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().StringVar(&specPath, "spec", "distro/soda.toml", "path to the Soda distribution specification")
	root.AddCommand(imageStageCommand(&specPath, connect), imagePromoteCommand(&specPath, connect), recordSignCommand(&specPath, connect))
	root.AddCommand(draftCommand(&specPath, connect), uploadCommand(&specPath, connect), publishCommand(&specPath, connect))
	return root
}
