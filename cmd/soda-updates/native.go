package main

import (
	"context"
	"errors"
	"io"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/LevitateOS/soda-os/internal/updates"
)

// nativeUpdates keeps query output separate from mutation progress and serializes
// only Soda mutations. Native bootc commands remain outside this lock.
type nativeUpdates struct {
	queries    process.Runner
	releases   *updates.Releases
	operations updates.Operations
	lock       func() (io.Closer, error)
}

func (native nativeUpdates) Status(ctx context.Context) (updates.Host, error) {
	return updates.ReadHost(ctx, native.queries)
}

func (native nativeUpdates) Check(ctx context.Context) (updates.Release, error) {
	return native.releases.Latest(ctx, native.operations.Architecture)
}

func (native nativeUpdates) Download(ctx context.Context, selection updates.Selection) error {
	return native.mutate(ctx, selection, native.operations.Download)
}

func (native nativeUpdates) Apply(ctx context.Context, selection updates.Selection) error {
	return native.mutate(ctx, selection, native.operations.Apply)
}

func (native nativeUpdates) mutate(ctx context.Context, selection updates.Selection, operation func(context.Context, updates.Selection) error) (resultErr error) {
	lock, err := native.lock()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, lock.Close()) }()
	return operation(ctx, selection)
}
