package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/LevitateOS/soda-os/internal/build/image"
	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/stretchr/testify/require"
)

type recordingArtifacts struct {
	ctx     context.Context
	options any
}

func (fake *recordingArtifacts) Build(ctx context.Context, options installer.Options) (string, error) {
	fake.ctx, fake.options = ctx, options
	return "installer.iso", nil
}

func (fake *recordingArtifacts) BuildQCOW2(ctx context.Context, options installer.QCOW2Options) (installer.QCOW2Result, error) {
	fake.ctx, fake.options = ctx, options
	return installer.QCOW2Result{}, nil
}

func (fake *recordingArtifacts) CreateRecord(ctx context.Context, options release.RecordOptions) (release.Result, error) {
	fake.ctx, fake.options = ctx, options
	return release.Result{}, nil
}

func TestNativeInstallerDefaultsAndOverridesRemainLocal(t *testing.T) {
	builder := &image.Builder{}
	builder.Spec.Platform.Installer.ToolLock = "platform.lock"
	fake := &recordingArtifacts{}
	native := nativeImage{Builder: builder, installer: fake}
	for _, lock := range []string{"", "custom.lock"} {
		expected := lock
		if expected == "" {
			expected = "platform.lock"
		}
		iso := installer.Options{ArchivePath: "archive", ToolLock: lock, OutputDir: "images"}
		_, err := native.BuildISO(t.Context(), iso)
		require.NoError(t, err)
		require.Equal(t, installer.Options{ArchivePath: "archive", ToolLock: expected, OutputDir: "images"}, fake.options)
		require.Equal(t, lock, iso.ToolLock)
		qcow := installer.QCOW2Options{ArchivePath: "archive", ToolLock: lock, OutputDir: "images"}
		_, err = native.BuildQCOW2(t.Context(), qcow)
		require.NoError(t, err)
		require.Equal(t, installer.QCOW2Options{ArchivePath: "archive", ToolLock: expected, OutputDir: "images"}, fake.options)
		require.Equal(t, lock, qcow.ToolLock)
		require.Equal(t, t.Context(), fake.ctx)
	}
}

func TestNativeRecordDefaultsUseSelectedPlatform(t *testing.T) {
	for _, architecture := range []string{"aarch64", "x86_64"} {
		builder := &image.Builder{}
		builder.Spec.Platform.Architecture.Artifact = architecture
		builder.Spec.Platform.Installer.ToolLock = architecture + ".lock"
		fake := &recordingArtifacts{}
		calls := 0
		native := nativeImage{Builder: builder, publisher: func() (recordPublisher, error) {
			calls++
			return fake, nil
		}}
		options := release.RecordOptions{ArchivePath: "image"}
		_, err := native.CreateRecord(t.Context(), options)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		require.Equal(t, release.RecordOptions{
			ArchivePath:       "image",
			InstallerArchive:  filepath.Join(".artifacts", "installer", "soda-installer-environment-"+architecture+".oci.tar"),
			InstallerToolLock: architecture + ".lock",
		}, fake.options)
		require.Empty(t, options.InstallerArchive)
		require.Empty(t, options.InstallerToolLock)
		options.InstallerArchive, options.InstallerToolLock = "custom.oci", "custom.lock"
		_, err = native.CreateRecord(t.Context(), options)
		require.NoError(t, err)
		require.Equal(t, options, fake.options)
		require.Equal(t, t.Context(), fake.ctx)
	}
}

func TestNativeRecordFactoryErrorIsPreserved(t *testing.T) {
	failure := errors.New("invalid publication specification")
	native := nativeImage{publisher: func() (recordPublisher, error) { return nil, failure }}
	_, err := native.CreateRecord(t.Context(), release.RecordOptions{InstallerArchive: "custom", InstallerToolLock: "custom"})
	require.ErrorIs(t, err, failure)
}
