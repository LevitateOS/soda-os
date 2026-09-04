package projects

import (
	"bytes"
	"encoding/json"
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

func TestCatalogPersistsRequiredFieldsInSortedEntries(t *testing.T) {
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

func TestCatalogPreservesAdditionalFieldsAcrossKnownFieldEdits(t *testing.T) {
	catalog := testCatalog(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(catalog.Path), 0o755))
	require.NoError(t, os.WriteFile(catalog.Path, []byte(`[
		{"id":"site","display_name":"Site","canonical_url":"git@git.test:site.git","notes":{"owner":"team"},"labels":["web"]}
	]`), 0o644))

	entry, err := catalog.Get("site")
	require.NoError(t, err)
	require.JSONEq(t, `{"owner":"team"}`, string(entry.Additional["notes"]))
	require.NoError(t, catalog.Edit(CatalogEntry{ID: "site", DisplayName: "New Site", CanonicalURL: entry.CanonicalURL}))

	contents, err := os.ReadFile(catalog.Path)
	require.NoError(t, err)
	require.JSONEq(t, `[{"id":"site","display_name":"New Site","canonical_url":"git@git.test:site.git","notes":{"owner":"team"},"labels":["web"]}]`, string(contents))
}

func TestCatalogRejectsDuplicateAndMissingFields(t *testing.T) {
	for name, contents := range map[string]string{
		"duplicate": `[{"id":"site","id":"other","display_name":"Site","canonical_url":"git@git.test:site.git"}]`,
		"missing":   `[{"id":"site","display_name":"Site"}]`,
		"id":        `[{"id":"site","display_name":"Site","canonical_url":"git@git.test:site.git"},{"id":"site","display_name":"Other","canonical_url":"git@git.test:other.git"}]`,
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

func TestDecodeCatalogMutationRejectsDuplicateAndAlternateWireShapes(t *testing.T) {
	for _, input := range []string{
		`{"id":"site","id":"other","display_name":"Site","canonical_url":"https://git.test/site"}`,
		`[]`,
	} {
		var request AddExistingRequest
		require.Error(t, DecodeRequest(strings.NewReader(input), &request))
	}
}

func TestDecodeCatalogMutationPreservesAdditionalFields(t *testing.T) {
	var request AddExistingRequest
	require.NoError(t, DecodeRequest(strings.NewReader(
		`{"id":"site","display_name":"Site","canonical_url":"git@git.test:site.git","owner":"alice","labels":["web"]}`,
	), &request))
	require.JSONEq(t, `"alice"`, string(request.Additional["owner"]))
	require.JSONEq(t, `["web"]`, string(request.Additional["labels"]))

	encoded, err := json.Marshal(request)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"site","display_name":"Site","canonical_url":"git@git.test:site.git","owner":"alice","labels":["web"]}`, string(encoded))
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
