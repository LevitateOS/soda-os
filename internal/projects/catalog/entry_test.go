package catalog

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEntryAcceptsSSHRemotes(t *testing.T) {
	t.Parallel()
	for _, remote := range []string{
		"ssh://git@git.example.test/team/site.git",
		"ssh://git.example.test/team/site.git",
		"git@git.example.test:team/site.git",
		"git.example.test:team/site.git",
		"git@[2001:db8::1]:team/site.git",
		"[2001:db8::1]:team/site.git",
	} {
		remote := remote
		t.Run(remote, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, (Entry{ID: "site", DisplayName: "Site", CanonicalURL: remote}).Validate())
		})
	}
}

func TestEntryRejectsCredentialsAndNonRemotePaths(t *testing.T) {
	t.Parallel()
	for _, remote := range []string{
		"", "team/site", "/srv/site", "file:///srv/site", "file:/srv/site", "FILE:relative", "C:/site", `D:\\site`, "ftp://git.example.test/site",
		"https://alice@git.example.test/site", "https://alice:secret@git.example.test/site",
		"https://git.example.test/site?token=secret", "https://git.example.test/site?",
		"https://git.example.test/site#", "ssh://git:secret@git.example.test/site",
		"git@git.example.test:team/site.git\x07",
	} {
		remote := remote
		t.Run(remote, func(t *testing.T) {
			t.Parallel()
			require.Error(t, (Entry{ID: "site", DisplayName: "Site", CanonicalURL: remote}).Validate())
		})
	}
}

func TestEntryRoundTripPreservesArbitraryMetadata(t *testing.T) {
	input := []byte(`{"id":"site","display_name":"Site","canonical_url":"git@git.test:site.git","owner":"alice","labels":["web"]}`)
	var entry Entry
	require.NoError(t, json.Unmarshal(input, &entry))
	require.JSONEq(t, `"alice"`, string(entry.Additional["owner"]))
	require.JSONEq(t, `["web"]`, string(entry.Additional["labels"]))

	encoded, err := json.Marshal(entry)
	require.NoError(t, err)
	require.JSONEq(t, string(input), string(encoded))
}

func TestEntryStrictlyRejectsDuplicateMissingAndTrailingValues(t *testing.T) {
	for name, input := range map[string]string{
		"duplicate": `{"id":"site","id":"other","display_name":"Site","canonical_url":"git@git.test:site.git"}`,
		"missing":   `{"id":"site","display_name":"Site"}`,
		"array":     `[]`,
		"trailing":  `{"id":"site","display_name":"Site","canonical_url":"git@git.test:site.git"} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			var entry Entry
			require.Error(t, json.Unmarshal([]byte(input), &entry))
		})
	}
}

func TestEditPreservesCanonicalURLAndRejectsItsInjection(t *testing.T) {
	current := Entry{
		ID:           "site",
		DisplayName:  "Site",
		CanonicalURL: "git@git.test:site.git",
		Additional:   map[string]json.RawMessage{"owner": json.RawMessage(`"alice"`)},
	}

	updated := (Edit{ID: "site", DisplayName: "New Site"}).Apply(current)
	require.Equal(t, current.CanonicalURL, updated.CanonicalURL)
	require.JSONEq(t, `"alice"`, string(updated.Additional["owner"]))

	var edit Edit
	require.ErrorContains(t, json.Unmarshal([]byte(
		`{"id":"site","display_name":"New Site","canonical_url":"git@git.test:site.git"}`,
	), &edit), `must not include "canonical_url"`)
}

func TestEditRoundTripSupportsArbitraryMetadata(t *testing.T) {
	input := []byte(`{"id":"site","display_name":"New Site","owner":{"team":"web"},"labels":["service"]}`)
	var edit Edit
	require.NoError(t, json.Unmarshal(input, &edit))
	require.JSONEq(t, `{"team":"web"}`, string(edit.Additional["owner"]))
	require.JSONEq(t, `["service"]`, string(edit.Additional["labels"]))

	encoded, err := json.Marshal(edit)
	require.NoError(t, err)
	require.JSONEq(t, string(input), string(encoded))
}

func TestEditStrictlyRejectsDuplicateFields(t *testing.T) {
	var edit Edit
	require.ErrorContains(t, json.Unmarshal([]byte(
		`{"id":"site","id":"other","display_name":"Site"}`,
	), &edit), `duplicate catalog field "id"`)
}
