package main

import (
	"context"
	"testing"

	"github.com/LevitateOS/soda-os/internal/build/image"
	"github.com/LevitateOS/soda-os/internal/build/installer"
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
