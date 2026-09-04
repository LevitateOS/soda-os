package projects

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLifecycleMutationHoldsTheSingleProjectsLock(t *testing.T) {
	catalog := testCatalog(t)
	require.NoError(t, catalog.Add(CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	_, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], "site")
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	platform.onDelete = func(Account) {
		close(entered)
		<-release
	}
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}
	removeResult := make(chan error, 1)
	go func() { removeResult <- lifecycle.RemoveProject(context.Background(), "alice", "site") }()
	<-entered
	addResult := make(chan error, 1)
	go func() {
		addResult <- catalog.Add(CatalogEntry{ID: "other", DisplayName: "Other", CanonicalURL: "git@git.example.test:other.git"})
	}()
	select {
	case err := <-addResult:
		t.Fatalf("catalog mutation escaped the root lifecycle lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-removeResult)
	require.NoError(t, <-addResult)
}
