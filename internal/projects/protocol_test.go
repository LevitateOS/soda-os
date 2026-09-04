package projects

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/LevitateOS/soda-os/internal/strictjson"
	"github.com/stretchr/testify/require"
)

func TestProjectViewKeepsArbitraryCatalogMetadataSeparate(t *testing.T) {
	t.Parallel()
	view := newProjectView(
		catalog.Entry{
			ID:           "site",
			DisplayName:  "Site",
			CanonicalURL: "git@git.example.test:team/site.git",
			Additional: map[string]json.RawMessage{
				"team":               json.RawMessage(`"web"`),
				"catalog_metadata":   json.RawMessage(`"catalog-value"`),
				"workspace_username": json.RawMessage(`"catalog-user"`),
				"workspace_exists":   json.RawMessage(`"catalog-exists"`),
			},
		},
		"soda-w-0123456789abcdef01234567",
		true,
	)

	contents, err := json.Marshal(view)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id":"site",
		"display_name":"Site",
		"canonical_url":"git@git.example.test:team/site.git",
		"catalog_metadata":{
			"team":"web",
			"catalog_metadata":"catalog-value",
			"workspace_username":"catalog-user",
			"workspace_exists":"catalog-exists"
		},
		"workspace_username":"soda-w-0123456789abcdef01234567",
		"workspace_exists":true
	}`, string(contents))
}

func TestProjectViewUsesAnEmptyMetadataObject(t *testing.T) {
	t.Parallel()
	contents, err := json.Marshal(newProjectView(
		catalog.Entry{
			ID:           "site",
			DisplayName:  "Site",
			CanonicalURL: "git@git.example.test:team/site.git",
		},
		"soda-w-0123456789abcdef01234567",
		false,
	))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id":"site",
		"display_name":"Site",
		"canonical_url":"git@git.example.test:team/site.git",
		"catalog_metadata":{},
		"workspace_username":"soda-w-0123456789abcdef01234567",
		"workspace_exists":false
	}`, string(contents))
}

func TestCatalogRequestsUseCatalogOwnedWireValues(t *testing.T) {
	t.Parallel()
	var add AddExistingRequest
	require.NoError(t, strictjson.Decode(strings.NewReader(
		`{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:team/site.git","team":"web","labels":["public"]}`,
	), &add))
	require.JSONEq(t, `"web"`, string(add.Additional["team"]))
	require.JSONEq(t, `["public"]`, string(add.Additional["labels"]))

	encoded, err := json.Marshal(HelperCatalogRequest(add))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id":"site",
		"display_name":"Site",
		"canonical_url":"git@git.example.test:team/site.git",
		"team":"web",
		"labels":["public"]
	}`, string(encoded))
}

func TestEditRequestsRejectCanonicalURLAtBothBoundaries(t *testing.T) {
	t.Parallel()
	for _, decode := range []struct {
		name        string
		destination any
	}{
		{name: "public", destination: new(EditRequest)},
		{name: "helper", destination: new(HelperEditRequest)},
	} {
		decode := decode
		t.Run(decode.name, func(t *testing.T) {
			t.Parallel()
			err := strictjson.Decode(strings.NewReader(
				`{"id":"site","display_name":"Renamed","canonical_url":"git@git.example.test:team/site.git"}`,
			), decode.destination)
			require.ErrorContains(t, err, `must not include "canonical_url"`)
		})
	}
}

func TestActionSpecificResponsesExposeOnlyTheirContract(t *testing.T) {
	t.Parallel()
	view := newProjectView(catalog.Entry{
		ID:           "site",
		DisplayName:  "Site",
		CanonicalURL: "git@git.example.test:team/site.git",
	}, "", false)
	cases := []struct {
		name     string
		response any
		expected string
	}{
		{name: "project mutation", response: ProjectMutationResponse{OK: true, Project: view}, expected: `{"ok":true,"project":{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:team/site.git","catalog_metadata":{},"workspace_username":"","workspace_exists":false}}`},
		{name: "setup", response: SetupResponse{OK: true, WorkspaceUsername: "soda-w-example"}, expected: `{"ok":true,"workspace_username":"soda-w-example"}`},
		{name: "success", response: SuccessResponse{OK: true}, expected: `{"ok":true}`},
		{name: "workspace preparation", response: WorkspacePreparationResponse{OK: true, WorkspaceUsername: "soda-w-example", WorkspacePublicKey: "ssh-ed25519 AAAA"}, expected: `{"ok":true,"workspace_username":"soda-w-example","workspace_public_key":"ssh-ed25519 AAAA"}`},
		{name: "workspace publication", response: WorkspacePublicationResponse{OK: true, WorkspaceUsername: "soda-w-example"}, expected: `{"ok":true,"workspace_username":"soda-w-example"}`},
	}
	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			contents, err := json.Marshal(test.response)
			require.NoError(t, err)
			require.JSONEq(t, test.expected, string(contents))
		})
	}
}
