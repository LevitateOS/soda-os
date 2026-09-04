package runners

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForgejoConfigurationUsesNativeTokenFileAndOneHostSlot(t *testing.T) {
	state := t.TempDir()
	native := NewNative()
	request := CreateRequest{
		ID: "forgejo-one", Provider: ProviderForgejo,
		RegistrationURL: "http://soda.example.test:30000",
		RegistrationID:  "33834eef-e758-48c4-a676-1745426747aa",
		Labels:          "soda-arm64:host", RegistrationToken: "provider-input",
	}
	require.NoError(t, native.configureForgejo(state, identity{uint32(os.Getuid()), uint32(os.Getgid())}, request))

	contents, err := os.ReadFile(filepath.Join(state, "forgejo-runner.yml"))
	require.NoError(t, err)
	require.NotContains(t, string(contents), request.RegistrationToken)
	var configuration map[string]any
	require.NoError(t, json.Unmarshal(contents, &configuration))
	runner := configuration["runner"].(map[string]any)
	require.Equal(t, float64(1), runner["capacity"])
	require.Equal(t, []any{"soda-arm64:host"}, runner["labels"])
	require.Equal(t, map[string]any{"enabled": false}, configuration["cache"])

	token, err := os.ReadFile(filepath.Join(state, "forgejo-token"))
	require.NoError(t, err)
	require.Equal(t, request.RegistrationToken+"\n", string(token))
	info, err := os.Stat(filepath.Join(state, "forgejo-token"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
