package main

import (
	"context"

	"github.com/LevitateOS/soda-os/internal/build/image"
	"github.com/LevitateOS/soda-os/internal/build/installer"
)

type installerOperations interface {
	Build(context.Context, installer.Options) (string, error)
	BuildQCOW2(context.Context, installer.QCOW2Options) (installer.QCOW2Result, error)
}

// nativeImage composes image and installer owners using the selected native
// specification and the supplied runner, without another build state model.
type nativeImage struct {
	*image.Builder
	installer installerOperations
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
