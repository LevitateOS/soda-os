package acceptance

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFallbackUsesExplicitDigestsAndPreservesCaptureLabels(t *testing.T) {
	evidence := prepareGuestCommands(t)
	cleanup := &Cleanup{}
	guest := enrolledTestGuest(t, evidence, cleanup)
	admin := personFixture{Remote: Remote{Evidence: evidence, KnownHosts: filepath.Join(t.TempDir(), "known-hosts")}, LinuxPassword: []byte("fixture-password")}
	t.Setenv("BOOTED_DIGEST", filepath.Join(t.TempDir(), "digest"))
	t.Setenv("BOOTC_COMMANDS", filepath.Join(t.TempDir(), "commands"))
	installAcceptanceCommand(t, "ssh-keyscan", "printf 'fixture-host-key\n'\n")
	installAcceptanceCommand(t, "curl", "exit 0\n")
	installAcceptanceCommand(t, "ssh", `input=$(cat)
for command do :; done
eval "set -- $command"
case "$*" in
 *--download-only*)
  for arg do
   case "$arg" in *@sha256:*) printf '%s' "${arg##*@}" >"$BOOTED_DIGEST" ;; esac
  done
  printf 'download:%s\n' "$(cat "$BOOTED_DIGEST")" >>"$BOOTC_COMMANDS" ;;
 *--from-downloaded*) printf 'activate\n' >>"$BOOTC_COMMANDS" ;;
 *bootc*status*)
  printf 'status\n' >>"$BOOTC_COMMANDS"
  printf '{"status":{"booted":{"image":{"imageDigest":"%s"}}}}' "$(cat "$BOOTED_DIGEST")" ;;
 *)
  case "$input" in
   *'/usr/bin/tailscale logout'*) printf 'logout\n' >>"$GUEST_EVENT_LOG" ;;
   *) printf 'stable-snapshot\n' ;;
  esac ;;
esac
`)
	candidateDigest, fallbackDigest := "sha256:"+strings.Repeat("b", 64), "sha256:"+strings.Repeat("a", 64)
	candidate := guestImageReference(5001, releaseRecord{SodaImageReference: "example.test/soda@" + candidateDigest})
	fallback := guestImageReference(5001, releaseRecord{SodaImageReference: "example.test/soda@" + fallbackDigest})
	require.Equal(t, "10.0.2.2:5001/soda-os@"+candidateDigest, candidate)
	require.Equal(t, "10.0.2.2:5001/soda-os@"+fallbackDigest, fallback)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, exerciseFallback(ctx, admin, guest, candidate, fallback))
	require.NotContains(t, guestEvents(t), "logout")
	commands, err := os.ReadFile(os.Getenv("BOOTC_COMMANDS"))
	require.NoError(t, err)
	require.Equal(t, "download:"+fallbackDigest+"\nactivate\nstatus\ndownload:"+candidateDigest+"\nactivate\nstatus\n", string(commands))
	for _, label := range []string{"b-before", "a-selected", "b-restored"} {
		snapshot, err := os.ReadFile(filepath.Join(evidence.Root, "fallback", label+".stdout"))
		require.NoError(t, err)
		require.Equal(t, "stable-snapshot\n", string(snapshot))
	}
	require.NoError(t, guest.shutdown(ctx))
	require.NoError(t, cleanup.Run(context.Background()))
}
