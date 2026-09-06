package main

import (
	"context"
	"errors"
	"io"
	"testing"

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
	err := native.mutate(ctx, func(received context.Context) error {
		require.Same(t, ctx, received)
		require.ErrorIs(t, received.Err(), context.Canceled)
		require.False(t, lock.closed)
		return operationErr
	})
	require.True(t, lock.closed)
	require.ErrorIs(t, err, operationErr)
	require.ErrorIs(t, err, closeErr)
}

func TestCheckAndUpdateUseTheSodaOperationLock(t *testing.T) {
	failure := errors.New("another operation is running")
	native := nativeUpdates{lock: func() (io.Closer, error) { return nil, failure }}
	// No runners are configured: a failed lock must stop before any native work.
	_, err := native.Check(t.Context())
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, native.Update(t.Context()), failure)
}

func TestMutationDoesNotRunWithoutLock(t *testing.T) {
	failure := errors.New("busy")
	native := nativeUpdates{lock: func() (io.Closer, error) { return nil, failure }}
	err := native.mutate(t.Context(), func(context.Context) error {
		t.Fatal("mutation must not run without its lock")
		return nil
	})
	require.ErrorIs(t, err, failure)
}
