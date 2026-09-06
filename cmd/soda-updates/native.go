package main

import (
	"context"
	"errors"
	"io"

	"github.com/LevitateOS/soda-os/internal/updates"
)

// nativeUpdates serializes Soda checks and updates, not administrator bootc
// commands. Status remains readable while an operation runs.
type nativeUpdates struct {
	operations updates.Operations
	lock       func() (io.Closer, error)
}

func (native nativeUpdates) Status(ctx context.Context) (updates.Host, error) {
	return native.operations.Status(ctx)
}

func (native nativeUpdates) Check(ctx context.Context) (updates.Host, error) {
	var host updates.Host
	err := native.mutate(ctx, func(ctx context.Context) error {
		var err error
		host, err = native.operations.Check(ctx)
		return err
	})
	return host, err
}

func (native nativeUpdates) Update(ctx context.Context) error {
	return native.mutate(ctx, native.operations.Update)
}

func (native nativeUpdates) mutate(ctx context.Context, operation func(context.Context) error) (resultErr error) {
	lock, err := native.lock()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, lock.Close()) }()
	return operation(ctx)
}
