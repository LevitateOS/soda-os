package catalog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogLockHonorsCancellationWithoutMutation(t *testing.T) {
	store := newTestStore(t)
	held, err := store.Lock(t.Context())
	require.NoError(t, err)
	defer held.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	lock, err := store.Lock(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, lock)
	entries, err := store.List()
	require.NoError(t, err)
	require.Empty(t, entries)
}
