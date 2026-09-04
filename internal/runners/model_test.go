package runners

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateRequestAcceptsOnlyTheTwoNativeProviderShapes(t *testing.T) {
	forgejo := CreateRequest{
		ID: "forgejo-one", Provider: ProviderForgejo,
		RegistrationURL: "http://soda.example.test:30000",
		RegistrationID:  "33834eef-e758-48c4-a676-1745426747aa",
		Labels:          "soda-arm64:host", RegistrationToken: "provider-input",
	}
	require.NoError(t, forgejo.Validate())

	github := CreateRequest{
		ID: "github-one", Provider: ProviderGitHub,
		RegistrationURL: "https://github.com/LevitateOS/soda-os",
		Labels:          "soda-local", RegistrationToken: "provider-input",
	}
	require.NoError(t, github.Validate())
	github.Labels = ""
	require.ErrorContains(t, github.Validate(), "GitHub labels")
	github.Labels = "soda-local"

	github.RegistrationURL = "https://git.example.test/team/repository"
	require.ErrorContains(t, github.Validate(), "github.com")
	forgejo.Labels = "container:docker://example.test/image"
	require.ErrorContains(t, forgejo.Validate(), "name:host")
}

func TestRunnerIdentityHasAStableNarrowLinuxShape(t *testing.T) {
	account, err := AccountName("build-arm64")
	require.NoError(t, err)
	require.Equal(t, "soda-runner-build-arm64", account)
	require.LessOrEqual(t, len(account), 32)
	require.Error(t, ValidateID("Project/runner"))
}
