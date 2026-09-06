package projects

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOperationLocksHonorCancelledWaits(t *testing.T) {
	locker, err := NewOperationLocker(testOperationLockFile(t), os.Getuid())
	require.NoError(t, err)
	held, err := locker.Exclusive(t.Context())
	require.NoError(t, err)
	defer held.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	shared, err := locker.Shared(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, shared)
	exclusive, err := locker.Exclusive(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, exclusive)
}

func TestCatalogHelperPassesCancellationToItsLock(t *testing.T) {
	fixture := newHelperFixture(t)
	held, err := fixture.store.Lock(t.Context())
	require.NoError(t, err)
	defer held.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = fixture.helper.Execute(ctx, helperAlice(), "catalog-add", strings.NewReader(
		`{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:site.git"}`,
	))
	require.ErrorIs(t, err, context.Canceled)
	entries, err := fixture.store.List()
	require.NoError(t, err)
	require.Empty(t, entries)
}
