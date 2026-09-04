package acceptance

import (
	"context"
	"errors"
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

func TestCleanupReportsFailureAndContinuesExactActions(t *testing.T) {
	completed := []string{}
	cleanup := &Cleanup{}
	require.NoError(t, cleanup.Add(CleanupAction{Name: "work", Run: func(context.Context) error {
		completed = append(completed, "work")
		return nil
	}}))
	require.NoError(t, cleanup.Add(CleanupAction{Name: "guest", Run: func(context.Context) error {
		completed = append(completed, "guest")
		return errors.New("native logout failure")
	}}))

	err := cleanup.Run(context.Background())
	require.ErrorContains(t, err, "cleanup guest: native logout failure")
	require.Equal(t, []string{"guest", "work"}, completed)
}
