package acceptance

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanupRunsExactRegisteredResourcesOnceInReverseOrder(t *testing.T) {
	var mu sync.Mutex
	completed := []string{}
	cleanup := &Cleanup{}
	for _, name := range []string{"work-directory", "registry", "qemu"} {
		name := name
		require.NoError(t, cleanup.Add(CleanupAction{Name: name, Run: func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			completed = append(completed, name)
			return nil
		}}))
	}

	var wait sync.WaitGroup
	for range 4 {
		wait.Add(1)
		go func() { defer wait.Done(); require.NoError(t, cleanup.Run(context.Background())) }()
	}
	wait.Wait()
	require.Equal(t, []string{"qemu", "registry", "work-directory"}, completed)
}
