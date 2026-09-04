package projects

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	root := t.TempDir()
	return &Catalog{Path: filepath.Join(root, "catalog", "projects.json"), LockPath: filepath.Join(root, "run", "projects.lock")}
}

func TestCatalogPersistsExactlyThreeSortedFields(t *testing.T) {
	catalog := testCatalog(t)
	require.NoError(t, catalog.Add(CatalogEntry{ID: "zebra", DisplayName: "Zebra", CanonicalURL: "git@git.example.test:zebra.git"}))
	require.NoError(t, catalog.Add(CatalogEntry{ID: "alpha", DisplayName: "Alpha", CanonicalURL: "git@git.example.test:alpha.git"}))
	contents, err := os.ReadFile(catalog.Path)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"id":"alpha","display_name":"Alpha","canonical_url":"git@git.example.test:alpha.git"},
		{"id":"zebra","display_name":"Zebra","canonical_url":"git@git.example.test:zebra.git"}
	]`, string(contents))
	info, err := os.Stat(catalog.Path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	require.NoError(t, catalog.Edit(CatalogEntry{ID: "alpha", DisplayName: "New Alpha", CanonicalURL: "ssh://git@git.example.test/alpha.git"}))
	require.NoError(t, catalog.Remove("zebra"))
	entries, err := catalog.List()
	require.NoError(t, err)
	require.Equal(t, []CatalogEntry{{ID: "alpha", DisplayName: "New Alpha", CanonicalURL: "ssh://git@git.example.test/alpha.git"}}, entries)
}

func TestCatalogRejectsExtraDuplicateAndMissingFields(t *testing.T) {
	for name, contents := range map[string]string{
		"extra":     `[{"id":"site","display_name":"Site","canonical_url":"https://git.test/site","owner":"alice"}]`,
		"duplicate": `[{"id":"site","id":"other","display_name":"Site","canonical_url":"https://git.test/site"}]`,
		"missing":   `[{"id":"site","display_name":"Site"}]`,
		"id":        `[{"id":"site","display_name":"Site","canonical_url":"https://git.test/site"},{"id":"site","display_name":"Other","canonical_url":"https://git.test/other"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			catalog := testCatalog(t)
			require.NoError(t, os.MkdirAll(filepath.Dir(catalog.Path), 0o755))
			require.NoError(t, os.WriteFile(catalog.Path, []byte(contents), 0o644))
			_, err := catalog.List()
			require.Error(t, err)
		})
	}
}

func TestDecodeRequestRejectsUnknownAndDuplicateFields(t *testing.T) {
	for _, input := range []string{
		`{"id":"site","id":"other","display_name":"Site","canonical_url":"https://git.test/site"}`,
		`{"id":"site","display_name":"Site","canonical_url":"https://git.test/site","owner":"alice"}`,
		`[]`,
	} {
		var request AddExistingRequest
		require.Error(t, DecodeRequest(strings.NewReader(input), &request))
	}
}

func TestCatalogAndRequestRejectInvalidUTF8Bytes(t *testing.T) {
	catalog := testCatalog(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(catalog.Path), 0o755))
	invalidCatalog := append([]byte(`[{"id":"site","display_name":"`), 0xff)
	invalidCatalog = append(invalidCatalog, []byte(`","canonical_url":"https://git.test/site"}]`)...)
	require.NoError(t, os.WriteFile(catalog.Path, invalidCatalog, 0o644))
	_, err := catalog.List()
	require.ErrorContains(t, err, "valid UTF-8")

	invalidRequest := append([]byte(`{"id":"site","display_name":"`), 0xff)
	invalidRequest = append(invalidRequest, []byte(`","canonical_url":"https://git.test/site"}`)...)
	var request AddExistingRequest
	require.ErrorContains(t, DecodeRequest(bytes.NewReader(invalidRequest), &request), "valid UTF-8")
}

func TestCatalogSerializesConcurrentMutations(t *testing.T) {
	catalog := testCatalog(t)
	var wait sync.WaitGroup
	for index := range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			id := fmt.Sprintf("project-%02d", index)
			require.NoError(t, catalog.Add(CatalogEntry{ID: id, DisplayName: id, CanonicalURL: "git@git.example.test:" + id + ".git"}))
		}()
	}
	wait.Wait()
	entries, err := catalog.List()
	require.NoError(t, err)
	require.Len(t, entries, 20)
}
