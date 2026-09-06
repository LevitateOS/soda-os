package runners

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeRunnerLockHonorsCancellation(t *testing.T) {
	native := &Native{LockPath: filepath.Join(t.TempDir(), "runners.lock")}
	held, err := native.lock(t.Context())
	require.NoError(t, err)
	defer held.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	lock, err := native.lock(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, lock)
}
