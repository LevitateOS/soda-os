package main

import (
	"context"

	"github.com/LevitateOS/soda-os/internal/build/release"
)

type recordingPublication struct {
	calls   []string
	ctx     context.Context
	options any
	err     error
}

func (fake *recordingPublication) record(ctx context.Context, action string, options any) error {
	fake.calls = append(fake.calls, action)
	fake.ctx, fake.options = ctx, options
	return fake.err
}

func (fake *recordingPublication) ImageStage(ctx context.Context, options release.ImageStageOptions) (release.ImageResult, error) {
	return release.ImageResult{}, fake.record(ctx, "image-stage", options)
}

func (fake *recordingPublication) ImagePromote(ctx context.Context, options release.ImagePromoteOptions) (release.ImageResult, error) {
	return release.ImageResult{}, fake.record(ctx, "image-promote", options)
}

func (fake *recordingPublication) SignRecord(ctx context.Context, options release.RecordSignOptions) (release.RecordSignResult, error) {
	return release.RecordSignResult{}, fake.record(ctx, "record-sign", options)
}

func (fake *recordingPublication) Draft(ctx context.Context, options release.DraftOptions) (release.PublicationResult, error) {
	return release.PublicationResult{}, fake.record(ctx, "draft", options)
}

func (fake *recordingPublication) Upload(ctx context.Context, options release.UploadOptions) (release.PublicationResult, error) {
	return release.PublicationResult{}, fake.record(ctx, "upload", options)
}

func (fake *recordingPublication) Publish(ctx context.Context, options release.PublishOptions) (release.PublicationResult, error) {
	return release.PublicationResult{}, fake.record(ctx, "publish", options)
}

type publicationCase struct {
	action  string
	flags   []string
	options any
	message string
}

func publicationCases() []publicationCase {
	return []publicationCase{
		{"image-stage", []string{"--architecture", "aarch64", "--archive", "image.oci"}, release.ImageStageOptions{Architecture: "aarch64", ArchivePath: "image.oci"}, "candidate:"},
		{"image-promote", []string{"--architecture", "x86_64", "--record", "release.json"}, release.ImagePromoteOptions{Architecture: "x86_64", RecordPath: "release.json"}, "Promoted"},
		{"record-sign", []string{"--architecture", "aarch64", "--record", "release.json"}, release.RecordSignOptions{Architecture: "aarch64", RecordPath: "release.json"}, "Signed"},
		{"draft", []string{"--notes-file", "notes.md", "--aarch64-record", "arm.json", "--x86_64-record", "x86.json"}, release.DraftOptions{NotesPath: "notes.md", AArch64RecordPath: "arm.json", X86RecordPath: "x86.json"}, "Created GitHub draft"},
		{"upload", []string{"--architecture", "x86_64", "--iso", "image.iso", "--qcow2-zst", "image.zst", "--record", "release.json", "--record-bundle", "bundle.json"}, release.UploadOptions{Architecture: "x86_64", ISOPath: "image.iso", QCOW2ZSTPath: "image.zst", RecordPath: "release.json", RecordBundlePath: "bundle.json"}, "Uploaded"},
		{"publish", []string{"--aarch64-record", "arm.json", "--x86_64-record", "x86.json"}, release.PublishOptions{AArch64RecordPath: "arm.json", X86RecordPath: "x86.json"}, "Published GitHub release"},
	}
}
