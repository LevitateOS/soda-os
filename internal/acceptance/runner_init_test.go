package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunInputsDoNotRequireAnUnusedEnrollmentSecret(t *testing.T) {
	for _, command := range []string{"cosign", "curl", "docker", "git", "qemu-img", "cloud-localds", "openssl", "ssh", "ssh-keygen", "ssh-keyscan"} {
		installAcceptanceCommand(t, command, "exit 0\n")
	}
	work := t.TempDir()
	private := filepath.Join(work, "private")
	public := filepath.Join(work, "public")
	password := filepath.Join(work, "password")
	require.NoError(t, os.WriteFile(private, []byte("fixture-private-key"), 0o600))
	require.NoError(t, os.WriteFile(public, []byte(testPublicKey), 0o644))
	require.NoError(t, os.WriteFile(password, []byte("fixture-password\n"), 0o600))
	options := defaultRunOptions(RunOptions{
		EvidenceDir:   filepath.Join(work, "evidence"),
		Administrator: AdministratorInput{PrivateKey: private, PublicKey: public, Password: password},
	})
	require.NoError(t, validateRunOptions(options))
	state := runnerState{paths: runPaths{work: work, adminKey: private}}
	require.NoError(t, state.loadSecrets([]byte("fixture-password")))
	labels := []string{}
	for _, secret := range state.secrets {
		labels = append(labels, secret.Label)
	}
	require.Equal(t, []string{"forgejo-owner-password", "administrator-password", "administrator-private-key"}, labels)
	// Removing the unused key must not relax protection of consumed credentials.
	require.NoError(t, os.Chmod(private, 0o644))
	require.ErrorContains(t, validateCredentialFiles(options), "administrator private key")
	require.NoError(t, os.Chmod(private, 0o600))
	require.NoError(t, os.Chmod(password, 0o644))
	require.ErrorContains(t, validateCredentialFiles(options), "administrator password")
}
