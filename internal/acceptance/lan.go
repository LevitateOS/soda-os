package acceptance

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

func (state *runnerState) localRemote() Remote {
	return Remote{
		Username: state.options.Administrator.Username, Host: "127.0.0.1", Port: state.options.Ports.SSH,
		CockpitPort: state.options.Ports.Cockpit, Key: state.paths.adminKey,
		KnownHosts: filepath.Join(state.paths.work, "iso-lan-known-hosts"), Evidence: state.evidence,
	}
}

func (state *runnerState) verifyInitialLAN(ctx context.Context, password []byte) error {
	local := state.localRemote()
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err := local.WaitReady(waitCtx); err != nil {
		return fmt.Errorf("verify LAN before enrollment: %w", err)
	}
	if err := local.Sudo(ctx, password, `set -euo pipefail
test "$(systemctl is-enabled firewalld.service)" = disabled
test "$(systemctl is-active firewalld.service || true)" = inactive
tailscale status --json | jq -e '.BackendState != "Running"' >/dev/null
`, "iso/lan-before-enrollment"); err != nil {
		return err
	}
	output, err := CommandOutput(ctx, CommandSpec{Name: "curl", Args: []string{
		"--fail", "--silent", "--show-error", "--max-time", "10",
		fmt.Sprintf("http://127.0.0.1:%d/api/healthz", state.options.Ports.Forgejo),
	}})
	if err != nil {
		return fmt.Errorf("verify Forgejo before enrollment: %w", err)
	}
	return state.evidence.Write("iso/lan-forgejo-before-enrollment.txt", output)
}
