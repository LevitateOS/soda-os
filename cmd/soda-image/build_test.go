package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/LevitateOS/soda-os/internal/build/oci"
	"github.com/stretchr/testify/require"
)

type recordingImage struct {
	calls   []string
	ctx     context.Context
	options any
	err     error
}

func (fake *recordingImage) record(ctx context.Context, action string, options any) error {
	fake.calls = append(fake.calls, action)
	fake.ctx, fake.options = ctx, options
	return fake.err
}

func (fake *recordingImage) Check(ctx context.Context) error {
	return fake.record(ctx, "check", nil)
}

func (fake *recordingImage) BuildRPMs(ctx context.Context) error {
	return fake.record(ctx, "rpm", nil)
}

func (fake *recordingImage) BuildImage(ctx context.Context, outputDir string) (string, error) {
	return "/actual/output/image.oci.tar", fake.record(ctx, "oci", outputDir)
}

func (fake *recordingImage) BuildISO(ctx context.Context, options installer.Options) (string, error) {
	return "installer.iso", fake.record(ctx, "iso", options)
}

func (fake *recordingImage) BuildQCOW2(ctx context.Context, options installer.QCOW2Options) (installer.QCOW2Result, error) {
	return installer.QCOW2Result{Path: "image.qcow2", SHA256: "checksum", CompressedPath: "image.qcow2.zst"}, fake.record(ctx, "qcow2", options)
}

func (fake *recordingImage) PublishImage(ctx context.Context, archive string) (oci.Image, error) {
	return oci.Image{Digest: "sha256:fixture", Revision: "archive-revision"}, fake.record(ctx, "publish", archive)
}

func TestOCIOutputDirectoryAndPathOnlyStdout(t *testing.T) {
	for _, directory := range []string{"", ".artifacts/images/x86_64/revision", "/absolute/output with spaces"} {
		t.Run(directory, func(t *testing.T) {
			fake := &recordingImage{}
			var output, progress bytes.Buffer
			command := newCommand(func(_, _ string, stdout, stderr io.Writer) (imageOperations, error) {
				require.Same(t, &progress, stdout)
				require.Same(t, &progress, stderr)
				_, err := fmt.Fprintln(stdout, "native progress is not an archive result")
				require.NoError(t, err)
				return fake, nil
			})
			command.SetOut(&output)
			command.SetErr(&progress)
			command.SetArgs([]string{"oci", "--architecture", "x86_64", "--output-dir", directory})
			require.NoError(t, command.ExecuteContext(t.Context()))
			require.Equal(t, directory, fake.options)
			require.Equal(t, t.Context(), fake.ctx)
			require.Equal(t, []string{"oci"}, fake.calls)
			require.Equal(t, "/actual/output/image.oci.tar\n", output.String())
			require.Equal(t, "native progress is not an archive result\n", progress.String())
		})
	}
}

type imageCase struct {
	action  string
	flags   []string
	options any
	message string
}

func imageCases() []imageCase {
	return []imageCase{
		{action: "check"},
		{action: "rpm"},
		{action: "oci", options: ".artifacts/images", message: "/actual/output/image.oci.tar\n"},
		{
			action:  "iso",
			flags:   []string{"--archive", "archive.oci"},
			options: installer.Options{ArchivePath: "archive.oci", OutputDir: ".artifacts/images"},
			message: "Built installer ISO: installer.iso\nChecksum: installer.iso.sha256\n",
		},
		{
			action:  "qcow2",
			flags:   []string{"--archive", "archive.oci"},
			options: installer.QCOW2Options{ArchivePath: "archive.oci", OutputDir: ".artifacts/images"},
			message: "Built QCOW2: image.qcow2\nChecksum: checksum\nCompressed: image.qcow2.zst\nChecksum: image.qcow2.zst.sha256\n",
		},
		{
			action:  "publish",
			flags:   []string{"--archive", "archive.oci"},
			options: "archive.oci",
			message: "Published " + oci.Repository + "@sha256:fixture from archive source archive-revision\n",
		},
	}
}
