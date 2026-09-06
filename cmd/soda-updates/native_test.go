package main

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/LevitateOS/soda-os/internal/updates"
	"github.com/stretchr/testify/require"
)

type recordingLock struct {
	closed bool
	err    error
}

func (lock *recordingLock) Close() error {
	lock.closed = true
	return lock.err
}

func TestMutationReleasesLockAndPreservesBothErrors(t *testing.T) {
	operationErr, closeErr := errors.New("operation failed"), errors.New("close failed")
	lock := &recordingLock{err: closeErr}
	native := nativeUpdates{lock: func() (io.Closer, error) { return lock, nil }}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := native.mutate(ctx, updates.Selection{}, func(received context.Context, _ updates.Selection) error {
		require.Same(t, ctx, received)
		require.ErrorIs(t, received.Err(), context.Canceled)
		require.False(t, lock.closed)
		return operationErr
	})
	require.True(t, lock.closed)
	require.ErrorIs(t, err, operationErr)
	require.ErrorIs(t, err, closeErr)
}

func TestMutationDoesNotRunWithoutLock(t *testing.T) {
	failure := errors.New("busy")
	native := nativeUpdates{lock: func() (io.Closer, error) { return nil, failure }}
	err := native.mutate(t.Context(), updates.Selection{}, func(context.Context, updates.Selection) error {
		t.Fatal("mutation must not run without its lock")
		return nil
	})
	require.ErrorIs(t, err, failure)
}
