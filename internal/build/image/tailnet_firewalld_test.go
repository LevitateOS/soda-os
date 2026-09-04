package image

import (
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTailnetIngressSurvivesFirewalldDefaultDrop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux network namespaces")
	}
	for _, binary := range []string{"unshare", "ip", "nft", "nc", "nsenter"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Skipf("requires %s", binary)
		}
	}

	// Tailscale's input chain accepts packets at filter priority. Firewalld
	// evaluates later, at filter + 10, so its default drop still wins unless
	// the tailscale interface is explicitly accepted in a firewalld zone.
	command := exec.Command("unshare", "-Urn", "bash", "-ceu", `
client=
receiver=
received=$(mktemp)
cleanup() {
  [ -z "$receiver" ] || kill "$receiver" 2>/dev/null || true
  [ -z "$client" ] || kill "$client" 2>/dev/null || true
  rm -f "$received"
}
trap cleanup EXIT

unshare -n --fork sleep 30 >/dev/null 2>&1 &
client=$!
sleep 0.1
ip link add client0 type veth peer name tailscale0
ip addr add 192.0.2.1/24 dev tailscale0
ip link set tailscale0 up
ip link set client0 netns "$client"
nsenter -t "$client" -n ip addr add 192.0.2.2/24 dev client0
nsenter -t "$client" -n ip link set lo up
nsenter -t "$client" -n ip link set client0 up

nft add table inet tailscale
nft add chain inet tailscale input '{ type filter hook input priority filter; policy accept; }'
nft add rule inet tailscale input iifname tailscale0 accept
nft add table inet firewalld
nft add chain inet firewalld input '{ type filter hook input priority filter + 10; policy accept; }'
nft add rule inet firewalld input drop

nc -l 192.0.2.1 30000 >"$received" &
receiver=$!
sleep 0.1
if nsenter -t "$client" -n sh -c 'printf blocked | nc -w 1 192.0.2.1 30000'; then
  echo "default drop did not block Tailscale ingress" >&2
  exit 1
fi
kill "$receiver" 2>/dev/null || true
receiver=

nft flush chain inet firewalld input
nft add rule inet firewalld input iifname tailscale0 accept
nft add rule inet firewalld input drop
nc -l 192.0.2.1 30000 >"$received" &
receiver=$!
sleep 0.1
nsenter -t "$client" -n sh -c 'printf allowed | nc -w 1 192.0.2.1 30000'
wait "$receiver"
receiver=
test "$(cat "$received")" = allowed
`)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}
