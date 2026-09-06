package main

import (
	"context"

	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/LevitateOS/soda-os/internal/build/release"
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

func (fake *recordingImage) BuildImage(ctx context.Context) error {
	return fake.record(ctx, "oci", nil)
}

func (fake *recordingImage) BuildISO(ctx context.Context, options installer.Options) (string, error) {
	return "installer.iso", fake.record(ctx, "iso", options)
}

func (fake *recordingImage) BuildQCOW2(ctx context.Context, options installer.QCOW2Options) (installer.QCOW2Result, error) {
	return installer.QCOW2Result{Path: "image.qcow2", SHA256: "checksum", CompressedPath: "image.qcow2.zst"}, fake.record(ctx, "qcow2", options)
}

func (fake *recordingImage) CreateRecord(ctx context.Context, options release.RecordOptions) (release.Result, error) {
	return release.Result{ImageReference: "reference", RecordPath: "record.json"}, fake.record(ctx, "record", options)
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
		{action: "oci"},
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
			action:  "record",
			flags:   []string{"--archive", "archive.oci", "--iso", "installer.iso", "--qcow2", "image.qcow2", "--qcow2-zst", "image.zst"},
			options: release.RecordOptions{ArchivePath: "archive.oci", ISOPath: "installer.iso", QCOW2Path: "image.qcow2", QCOW2ZSTPath: "image.zst", OutputDir: ".artifacts/releases"},
			message: "Recorded reference\nRelease record: record.json\n",
		},
	}
}
