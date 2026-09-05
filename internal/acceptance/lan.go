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
test -f /etc/cloud/cloud-init.disabled
rpm -q firewalld
firewall_enabled=$(systemctl is-enabled firewalld.service || true)
firewall_active=$(systemctl is-active firewalld.service || true)
printf 'firewalld.service is-enabled=%s\nfirewalld.service is-active=%s\n' "$firewall_enabled" "$firewall_active"
test "$firewall_enabled" = enabled
test "$firewall_active" = active
firewall-cmd --query-port=9090/tcp
firewall-cmd --permanent --query-port=9090/tcp
/usr/libexec/soda/soda-console-welcome | grep -F 'Firewall: enabled by default. Cockpit TCP port 9090 is allowed.'
tailscale status --json | jq -e '.BackendState != "Running"' >/dev/null
`, "iso/lan-before-enrollment"); err != nil {
		return err
	}
	// The suite administrator opens Forgejo only after checking first-boot defaults.
	if err := local.Sudo(ctx, password, acceptanceForgejoFirewall, "iso/administrator-allows-forgejo"); err != nil {
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
