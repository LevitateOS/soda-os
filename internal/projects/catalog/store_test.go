package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	store, err := NewStore(
		filepath.Join(root, "catalog", "projects.json"),
		filepath.Join(root, "run", "projects.lock"),
	)
	require.NoError(t, err)
	return store
}

func TestStoreConstructorsRequireExplicitInjectedPaths(t *testing.T) {
	_, err := NewStore("", "/run/catalog.lock")
	require.ErrorContains(t, err, "catalog path is required")
	_, err = NewStore("/var/lib/catalog.json", "")
	require.ErrorContains(t, err, "lock path is required")

	system := NewSystemStore()
	require.Equal(t, SystemPath, system.path)
	require.Equal(t, SystemLockPath, system.lockPath)

	var zero Store
	_, err = zero.List()
	require.Error(t, err, "the zero value must not select production paths")
}

func TestStorePersistsSortedEntriesWithRequiredMode(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Add(Entry{ID: "zebra", DisplayName: "Zebra", CanonicalURL: "git@git.example.test:zebra.git"}))
	require.NoError(t, store.Add(Entry{ID: "alpha", DisplayName: "Alpha", CanonicalURL: "git@git.example.test:alpha.git"}))

	contents, err := os.ReadFile(store.path)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"id":"alpha","display_name":"Alpha","canonical_url":"git@git.example.test:alpha.git"},
		{"id":"zebra","display_name":"Zebra","canonical_url":"git@git.example.test:zebra.git"}
	]`, string(contents))
	info, err := os.Stat(store.path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestStoreEditPreservesCanonicalURLAndReplacesMetadata(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Add(Entry{
		ID:           "site",
		DisplayName:  "Site",
		CanonicalURL: "git@git.test:site.git",
		Additional:   map[string]json.RawMessage{"owner": json.RawMessage(`"alice"`)},
	}))
	require.NoError(t, store.Edit(Edit{
		ID:          "site",
		DisplayName: "New Site",
		Additional:  map[string]json.RawMessage{"labels": json.RawMessage(`["web"]`)},
	}))

	entry, err := store.Get("site")
	require.NoError(t, err)
	require.Equal(t, "git@git.test:site.git", entry.CanonicalURL)
	require.NotContains(t, entry.Additional, "owner")
	require.JSONEq(t, `["web"]`, string(entry.Additional["labels"]))
}

func TestStoreRejectsDuplicateMissingAndInvalidCatalogData(t *testing.T) {
	for name, contents := range map[string][]byte{
		"duplicate field": []byte(`[{"id":"site","id":"other","display_name":"Site","canonical_url":"git@git.test:site.git"}]`),
		"missing field":   []byte(`[{"id":"site","display_name":"Site"}]`),
		"duplicate id":    []byte(`[{"id":"site","display_name":"Site","canonical_url":"git@git.test:site.git"},{"id":"site","display_name":"Other","canonical_url":"git@git.test:other.git"}]`),
		"alternate shape": []byte(`{"id":"site","display_name":"Site","canonical_url":"git@git.test:site.git"}`),
		"trailing value":  []byte(`[{"id":"site","display_name":"Site","canonical_url":"git@git.test:site.git"}] {}`),
		"invalid utf8":    append([]byte(`[{"id":"site","display_name":"`), 0xff),
	} {
		t.Run(name, func(t *testing.T) {
			store := newTestStore(t)
			require.NoError(t, os.MkdirAll(filepath.Dir(store.path), 0o755))
			require.NoError(t, os.WriteFile(store.path, contents, 0o644))
			_, err := store.List()
			require.Error(t, err)
		})
	}
}

func TestLockedStoreOwnsRemovalAndBlocksInternalMutations(t *testing.T) {
	store := newTestStore(t)
	require.NoError(t, store.Add(Entry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.test:site.git"}))

	locked, err := store.Lock()
	require.NoError(t, err)
	defer locked.Close()
	entry, err := locked.Get("site")
	require.NoError(t, err)
	require.Equal(t, "site", entry.ID)

	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(started)
		result <- store.Add(Entry{ID: "other", DisplayName: "Other", CanonicalURL: "git@git.test:other.git"})
	}()
	<-started
	select {
	case err = <-result:
		require.Failf(t, "concurrent add acquired held lock", "result: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, locked.Remove("site"))
	require.NoError(t, locked.Close())
	require.NoError(t, <-result)
	require.ErrorContains(t, locked.Remove("other"), "lock is closed")

	entries, err := store.List()
	require.NoError(t, err)
	require.Equal(t, []Entry{{ID: "other", DisplayName: "Other", CanonicalURL: "git@git.test:other.git"}}, entries)
}

func TestStoreSerializesConcurrentMutations(t *testing.T) {
	store := newTestStore(t)
	results := make(chan error, 20)
	var wait sync.WaitGroup
	for index := range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			id := fmt.Sprintf("project-%02d", index)
			results <- store.Add(Entry{ID: id, DisplayName: id, CanonicalURL: "git@git.example.test:" + id + ".git"})
		}()
	}
	wait.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}

	entries, err := store.List()
	require.NoError(t, err)
	require.Len(t, entries, 20)
}
