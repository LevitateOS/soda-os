package acceptance

const coreGuestChecks = `set -euo pipefail
test "$(id -u)" -ge 1000
id -nG | tr ' ' '\n' | grep -Fx wheel >/dev/null
test -s "$HOME/.ssh/authorized_keys"
ssh-keygen -l -f "$HOME/.ssh/authorized_keys" >/dev/null
test "$(getenforce)" = Enforcing
for unit in sshd cockpit.socket forgejo tailscaled; do
  test "$(systemctl is-active "$unit")" = active
done
rpm -q cockpit-ws cockpit-system cockpit-storaged cockpit-networkmanager soda-release soda-runtime soda-projects soda-forgejo soda-tea mise
for path in /usr/share/cockpit/storaged/manifest.json /usr/share/cockpit/networkmanager/manifest.json /usr/share/cockpit/soda-projects/manifest.json /usr/share/cockpit/soda-tailscale/manifest.json /usr/share/cockpit/soda-tailscale/app.mjs /usr/share/cockpit/branding/sodaos/branding.css; do
  test -s "$path"
done
for asset in branding.css palette.css theme.css soda-symbol.svg soda-logo-horizontal.svg soda-logo-horizontal-dark.svg login-background-light.svg login-background-dark.svg favicon.ico apple-touch-icon.png; do
  test -s "/usr/share/cockpit/branding/sodaos/$asset"
  rpm -qf "/usr/share/cockpit/branding/sodaos/$asset" | grep '^soda-projects-'
done
for page in systemd/logs systemd/services systemd/terminal systemd/hwinfo networkmanager/firewall; do
  grep -Fq '../../static/branding.css' "/usr/share/cockpit/$page.html"
done
for package in soda-projects soda-runners soda-tailscale; do
  cockpit-bridge --packages | awk '{print $1}' | grep -Fx "$package" >/dev/null
done
command -v git
command -v gh
command -v tea
command -v mise
test ! -e "$HOME/.config/tea/config.yml"
test ! -e "$HOME/.config/gh/hosts.yml"
test "$(systemctl is-enabled bootc-fetch-apply-updates.timer 2>/dev/null || true)" = masked
test "$(systemctl is-enabled rpm-ostree-countme.timer 2>/dev/null || true)" = masked
test "$(systemctl is-enabled firewalld.service)" = enabled
test "$(systemctl is-active firewalld.service)" = active
` + forbiddenServiceChecks + `for path in \
  /usr/libexec/soda/soda-setup \
  /usr/bin/soda-local-access \
  /var/lib/soda/setup-complete \
  /run/lock/soda/setup.lock \
  /var/lib/soda/soda.db \
  /var/lib/soda/built-in-git-token \
  /var/lib/soda/projects \
  /var/srv/soda/projects \
  /etc/soda/authorized_keys \
  /usr/libexec/soda/soda-authd \
  /usr/libexec/soda/soda-cockpit \
  /usr/libexec/soda/sodad \
  /usr/bin/sodactl \
  /run/soda/sodad.sock \
  /usr/libexec/soda/soda-installer-input \
  /usr/libexec/soda/soda-installer-finalize \
  /usr/libexec/soda/soda-cloud-finalize \
  /etc/cloud/cloud.cfg.d/99-soda-datasources.cfg \
	/var/lib/soda-install \
	/opt/soda/toolchains \
	/var/lib/soda/toolchains \
	/var/lib/soda/mise; do
  test ! -e "$path"
done
printf 'core-product-boundaries=pass\n'
`

const forbiddenServiceChecks = `for unit in soda-authd.service soda-cockpit.service sodad.service avahi-daemon.service var-srv-soda-projects.mount soda-tailscale-enroll.service soda-setup.service; do
  units=$(systemctl list-unit-files --no-legend --no-pager "$unit")
  test -z "$units"
done
`

// Explicit administrator configuration in disposable acceptance guests, not image defaults.
const acceptanceForgejoFirewall = `set -euo pipefail
test "$(systemctl is-enabled firewalld.service)" = enabled
test "$(systemctl is-active firewalld.service)" = active
firewall-cmd --query-port=9090/tcp
firewall-cmd --permanent --query-port=9090/tcp
for port in 30000/tcp 2222/tcp; do
  test "$(firewall-cmd --query-port="$port" || true)" = no
  test "$(firewall-cmd --permanent --query-port="$port" || true)" = no
done
firewall-cmd --permanent --add-port=30000/tcp --add-port=2222/tcp
firewall-cmd --add-port=30000/tcp --add-port=2222/tcp
`

const qcow2GuestChecks = `set -euo pipefail
tailscale status --json | jq -e '.BackendState != "Running"' >/dev/null
test -e /var/lib/cloud/instance
cloud-init status --wait
test ! -e /run/soda-installer
printf 'reusable-qcow2-cloud-init=pass\n'
`

const workspaceBoundaryChecks = `set -euo pipefail
primary=$1
project=$2
workspace=$3
primary_home=$(getent passwd "$primary" | cut -d: -f6)
workspace_home=$(getent passwd "$workspace" | cut -d: -f6)
test "$workspace_home" != "$primary_home"
cmp "$primary_home/.ssh/authorized_keys" "$workspace_home/.ssh/authorized_keys"
test -s "$workspace_home/.ssh/id_ed25519_soda"
test -s "$workspace_home/.ssh/id_ed25519_soda.pub"
test -d "$workspace_home/Projects/$project/.git"
test "$(runuser --user "$workspace" -- git -C "$workspace_home/Projects/$project" rev-parse --is-inside-work-tree)" = true
test "$(runuser --user "$workspace" -- git -C "$workspace_home/Projects/$project" rev-parse --git-common-dir)" = .git
test "$(stat -c %U "$workspace_home/.ssh/id_ed25519_soda")" = "$workspace"
test "$(stat -c %a "$workspace_home/.ssh/id_ed25519_soda")" = 600
test ! -e "$workspace_home/.config/tea/config.yml"
test ! -e "$workspace_home/.config/gh/hosts.yml"
test ! -e "$primary_home/.ssh/id_ed25519_soda"
runuser --user "$workspace" -- /bin/sh -c 'command -v git; command -v gh; command -v tea; command -v mise'
printf 'workspace=%s\n' "$workspace"
`

// Service state alone does not establish network reachability.
const nativeServiceChecks = `set -euo pipefail
test "$(systemctl is-active sshd)" = active
test "$(systemctl is-active cockpit.socket)" = active
test "$(systemctl is-active forgejo)" = active
printf 'native-service-state=pass\n'
`

const tailscaleAccessCheck = `set -euo pipefail
tailscale status --json | jq -e '.BackendState == "Running" and (.Self.Expired != true)' >/dev/null
printf 'tailscale-access=pass\n'
`
