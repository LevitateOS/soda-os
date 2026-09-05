package acceptance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreservationDetectsFixtureCorruption(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("guest scripts require native Linux tools; exercised by check-native")
	}
	for _, changed := range []string{"private", "tracked", "home", "permissions", "keys", "outbound", "password", "groups", "node", "catalog", "canonical"} {
		t.Run(changed, func(t *testing.T) {
			root, remote := preparePreservationFixture(t)
			capture := func(label string) ([]byte, error) {
				return remote.SudoOutput(context.Background(), []byte("password"), "set -- primary workspace kept\n"+stableManifestScript, label)
			}
			before, err := capture("before")
			require.NoError(t, err)
			same, err := capture("unchanged")
			require.NoError(t, err)
			require.NoError(t, compareManifests(before, same, "unchanged"))
			corruptPreservationFixture(t, root, changed)
			after, err := capture("after")
			if err == nil {
				require.Error(t, compareManifests(before, after, changed))
			}
		})
	}
}

func preparePreservationFixture(t *testing.T) (string, Remote) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("PRESERVATION_ROOT", root)
	t.Setenv("PRESERVATION_SHADOW", "password-one")
	t.Setenv("PRESERVATION_GROUPS", "users wheel")
	installRemoteShell(t)
	installAcceptanceCommand(t, "getent", `case "$1" in
 passwd) printf '%s:x:1000:1000::%s/%s:/bin/bash\n' "$2" "$PRESERVATION_ROOT" "$2" ;;
 shadow) printf '%s\n' "$PRESERVATION_SHADOW" ;;
esac`)
	installAcceptanceCommand(t, "id", `printf '%s\n' "$PRESERVATION_GROUPS"`)
	installAcceptanceCommand(t, "runuser", `shift 3; exec "$@"`)
	installAcceptanceCommand(t, "mise", `printf 'v22.14.0\n'`)
	installAcceptanceCommand(t, "tailscale", `printf '{"Self":{"ID":"guest","DNSName":"guest.test","TailscaleIPs":["100.64.0.1"]}}'`)
	installAcceptanceCommand(t, "nmcli", `printf 'native:ethernet:public\n'`)
	jq, err := exec.LookPath("jq")
	require.NoError(t, err)
	installAcceptanceCommand(t, "jq", `if test "$1" = -Sc; then exec `+remoteCommand([]string{jq})+` -Sc . "$PRESERVATION_ROOT/catalog.json"; fi
exec `+remoteCommand([]string{jq})+` "$@"`)
	sha, err := exec.LookPath("sha256sum")
	require.NoError(t, err)
	installAcceptanceCommand(t, "sha256sum", `case "$1" in /etc/ssh/*) printf 'host-key-hash\n' ;; *) exec `+remoteCommand([]string{sha})+` "$@" ;; esac`)
	for _, name := range []string{"primary/.ssh/authorized_keys", "workspace/.ssh/authorized_keys", "workspace/.ssh/id_ed25519_soda", "workspace/.ssh/id_ed25519_soda.pub", "workspace/soda-acceptance-state.txt", "workspace/.local/share/mise/installs/node/22.14.0/bin/node"} {
		writePreservationFile(t, filepath.Join(root, name), "original\n")
	}
	writePreservationFile(t, filepath.Join(root, "catalog.json"), `[{"id":"kept"}]`)
	seedPreservationGit(t, root)
	return root, testPerson(t, "owner").Remote
}

func seedPreservationGit(t *testing.T, root string) {
	t.Helper()
	checkout := filepath.Join(root, "workspace/Projects/kept")
	writePreservationFile(t, filepath.Join(checkout, "preserved.txt"), "committed\n")
	commands := [][]string{
		{"init", "--bare", "--initial-branch=main", filepath.Join(root, "origin.git")},
		{"init", "--initial-branch=main", checkout},
		{"-C", checkout, "config", "user.email", "acceptance@localhost"},
		{"-C", checkout, "config", "user.name", "Acceptance"},
		{"-C", checkout, "add", "preserved.txt"},
		{"-C", checkout, "-c", "commit.gpgsign=false", "commit", "-m", "fixture"},
		{"-C", checkout, "remote", "add", "origin", filepath.Join(root, "origin.git")},
		{"-C", checkout, "push", "origin", "main"},
	}
	for _, args := range commands {
		output, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, "%s", output)
	}
	writePreservationFile(t, filepath.Join(checkout, "preserved.txt"), "modified\n")
	writePreservationFile(t, filepath.Join(checkout, "private.txt"), "private\n")
}

func corruptPreservationFixture(t *testing.T, root, changed string) {
	t.Helper()
	files := map[string]string{
		"private": "workspace/Projects/kept/private.txt", "tracked": "workspace/Projects/kept/preserved.txt",
		"keys": "primary/.ssh/authorized_keys", "outbound": "workspace/.ssh/id_ed25519_soda",
		"node": "workspace/.local/share/mise/installs/node/22.14.0/bin/node", "catalog": "catalog.json",
	}
	switch changed {
	case "home":
		require.NoError(t, os.RemoveAll(filepath.Join(root, "workspace")))
	case "permissions":
		require.NoError(t, os.Chmod(filepath.Join(root, "workspace/Projects/kept/private.txt"), 0o644))
	case "password":
		t.Setenv("PRESERVATION_SHADOW", "different-password")
	case "groups":
		t.Setenv("PRESERVATION_GROUPS", "users")
	case "canonical":
		require.NoError(t, os.RemoveAll(filepath.Join(root, "origin.git/objects")))
	default:
		writePreservationFile(t, filepath.Join(root, files[changed]), "[]\n")
	}
}

func writePreservationFile(t *testing.T, path, contents string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
}
