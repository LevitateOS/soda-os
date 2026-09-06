package main

import (
	"context"
	"path/filepath"

	"github.com/LevitateOS/soda-os/internal/build/image"
	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/LevitateOS/soda-os/internal/build/release"
)

type installerOperations interface {
	Build(context.Context, installer.Options) (string, error)
	BuildQCOW2(context.Context, installer.QCOW2Options) (installer.QCOW2Result, error)
}

type recordPublisher interface {
	CreateRecord(context.Context, release.RecordOptions) (release.Result, error)
}

// nativeImage composes image, installer, and record owners using one selected
// specification and the same supplied runner. It introduces no new build state.
type nativeImage struct {
	*image.Builder
	installer installerOperations
	publisher func() (recordPublisher, error)
}

func (native nativeImage) BuildISO(ctx context.Context, options installer.Options) (string, error) {
	if options.ToolLock == "" {
		options.ToolLock = native.Spec.Platform.Installer.ToolLock
	}
	return native.installer.Build(ctx, options)
}

func (native nativeImage) BuildQCOW2(ctx context.Context, options installer.QCOW2Options) (installer.QCOW2Result, error) {
	if options.ToolLock == "" {
		options.ToolLock = native.Spec.Platform.Installer.ToolLock
	}
	return native.installer.BuildQCOW2(ctx, options)
}

func (native nativeImage) CreateRecord(ctx context.Context, options release.RecordOptions) (release.Result, error) {
	if options.InstallerArchive == "" {
		options.InstallerArchive = filepath.Join(".artifacts", "installer", "soda-installer-environment-"+native.Spec.Platform.Architecture.Artifact+".oci.tar")
	}
	if options.InstallerToolLock == "" {
		options.InstallerToolLock = native.Spec.Platform.Installer.ToolLock
	}
	publisher, err := native.publisher()
	if err != nil {
		return release.Result{}, err
	}
	return publisher.CreateRecord(ctx, options)
}
