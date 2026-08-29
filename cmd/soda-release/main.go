package main

import (
	"fmt"
	"os"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/spf13/cobra"
)

func main() {
	var specPath, tokenEnvironment, outputDir string
	var arm, x86 release.ReleaseArtifact
	command := &cobra.Command{
		Use:           "soda-release",
		Short:         "publish one paired Soda OS GitHub release",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(command *cobra.Command, _ []string) error {
			root, err := os.Getwd()
			if err != nil {
				return err
			}
			spec, err := config.LoadDistro(specPath, "aarch64")
			if err != nil {
				return err
			}
			publisher, err := release.NewPublisher(root, spec, nil)
			if err != nil {
				return err
			}
			result, err := publisher.PublishPaired(command.Context(), release.PairedPublicationOptions{AArch64: arm, X8664: x86, GitHubToken: os.Getenv(tokenEnvironment), OutputDir: outputDir})
			if err != nil {
				return err
			}
			fmt.Printf("Published GitHub release %s\nRelease index: %s\n", result.Tag, result.IndexPath)
			return nil
		},
	}
	command.Flags().StringVar(&specPath, "spec", "distro/soda.toml", "path to the Soda distribution specification")
	command.Flags().StringVar(&tokenEnvironment, "github-token-env", "SODA_GITHUB_TOKEN", "environment variable containing the GitHub release token")
	command.Flags().StringVar(&outputDir, "output-dir", ".artifacts/releases", "paired release output directory")
	command.Flags().StringVar(&arm.ISOPath, "aarch64-iso", "", "AArch64 installer ISO")
	command.Flags().StringVar(&arm.RecordPath, "aarch64-record", "", "AArch64 release record")
	command.Flags().StringVar(&x86.ISOPath, "x86_64-iso", "", "x86-64 installer ISO")
	command.Flags().StringVar(&x86.RecordPath, "x86_64-record", "", "x86-64 release record")
	for _, name := range []string{"aarch64-iso", "aarch64-record", "x86_64-iso", "x86_64-record"} {
		_ = command.MarkFlagRequired(name)
	}
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "soda-release:", err)
		os.Exit(1)
	}
}
