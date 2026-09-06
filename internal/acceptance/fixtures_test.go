package acceptance

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkspaceSetupUsesSuppliedPersonAndForgejoCredentials(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("SETUP_COUNT", filepath.Join(directory, "setup-count"))
	t.Setenv("KEY_REGISTRATION", filepath.Join(directory, "registration"))
	installAcceptanceCommand(t, "ssh", `for command do :; done
eval "set -- $command"
case "$*" in
 *soda-projects*setup)
  if test ! -e "$SETUP_COUNT"; then
   touch "$SETUP_COUNT"
   printf 'retained retry `+testPublicKey+`' >&2
   exit 1
  fi
  printf '%s' '{"ok":true,"workspace_username":"supplied-workspace"}' ;;
 *soda-projects*list) printf '%s' '{"projects":[{"id":"supplied-project","workspace_username":"supplied-workspace","workspace_exists":true}]}' ;;
 *curl*) cat >"$KEY_REGISTRATION" ;;
esac
`)
	person := testPerson(t, "supplied-person")
	workspace, err := setupWorkspace(context.Background(), person, "supplied-project", "setup")
	require.NoError(t, err)
	require.Equal(t, "supplied-workspace", workspace.Remote.Username)
	require.Equal(t, person.Remote.Key, workspace.Remote.Key)
	require.Equal(t, person.Remote.Host, workspace.Remote.Host)
	require.Equal(t, "supplied-project", workspace.ProjectID)
	require.Equal(t, person, workspace.Person)
	registration, err := os.ReadFile(os.Getenv("KEY_REGISTRATION"))
	require.NoError(t, err)
	require.Contains(t, string(registration), "supplied-person:linux-password")
}

func TestFailedSetupDoesNotReturnCompletedFixture(t *testing.T) {
	installAcceptanceCommand(t, "ssh", "exit 0\n")
	workspace, err := setupWorkspace(context.Background(), testPerson(t, "owner"), "kept", "setup")
	require.ErrorContains(t, err, "before its outbound Git key was registered")
	require.Equal(t, workspaceFixture{}, workspace)
}

func TestGeneratedFixtureKeysAreRegisteredForSanitization(t *testing.T) {
	installAcceptanceCommand(t, "ssh-keygen", `for path do :; done
printf private-fixture-material >"$path"
printf '`+testPublicKey+`' >"$path.pub"
`)
	secrets := []Secret{}
	keys := fixtureKeys{Directory: t.TempDir(), Secrets: &secrets}
	key, err := keys.generate(context.Background(), "person")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(keys.Directory, "person"), key.PrivatePath)
	require.Equal(t, []byte(testPublicKey), key.Public)
	require.Equal(t, []Secret{{Label: "person-private-key", Value: []byte("private-fixture-material")}}, secrets)
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	require.NoError(t, evidence.Write("leak", secrets[0].Value))
	require.ErrorContains(t, evidence.Sanitize(secrets), "redacted")
}

func TestFixtureKeyGenerationNeverReusesExistingMaterial(t *testing.T) {
	for _, suffix := range []string{"", ".pub"} {
		secrets := []Secret{}
		keys := fixtureKeys{Directory: t.TempDir(), Secrets: &secrets}
		path := filepath.Join(keys.Directory, "person") + suffix
		require.NoError(t, os.WriteFile(path, []byte("existing material"), 0o600))
		_, err := keys.generate(context.Background(), "person")
		require.ErrorContains(t, err, "must be new")
		require.Empty(t, secrets)
		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "existing material", string(contents))
	}
}

func testPerson(t *testing.T, username string) personFixture {
	t.Helper()
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	return personFixture{
		Remote:    Remote{Username: username, Host: "fixture-host", Port: 2222, Key: "fixture-incoming-key", Evidence: evidence},
		PublicKey: []byte(testPublicKey), LinuxPassword: []byte("linux-password"),
	}
}
