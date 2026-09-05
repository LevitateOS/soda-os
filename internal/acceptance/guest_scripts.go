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
test "$(systemctl is-enabled firewalld.service)" = disabled
test "$(systemctl is-active firewalld.service || true)" = inactive
for unit in soda-authd.service soda-cockpit.service sodad.service avahi-daemon.service var-srv-soda-projects.mount soda-tailscale-enroll.service soda-setup.service; do
  ! systemctl cat "$unit" >/dev/null 2>&1
done
for path in \
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

const stableManifestScript = `set -euo pipefail
accounts=$(
  getent passwd | awk -F: '$5 ~ /^soda-workspace=/ || $3 >= 1000 {print $1":"$3":"$4":"$5":"$6":"$7}' | LC_ALL=C sort | jq -Rsc 'split("\n") | map(select(length > 0))'
)
groups=$(
  getent group | LC_ALL=C sort | jq -Rsc 'split("\n") | map(select(length > 0))'
)
homes=$(
  getent passwd | awk -F: '$5 ~ /^soda-workspace=/ || $3 >= 1000 {print $1":"$6}' | while IFS=: read -r user home; do
    test -d "$home"
    shadow=$(getent shadow "$user" | sha256sum | cut -d' ' -f1)
    keys=absent
    test ! -f "$home/.ssh/authorized_keys" || keys=$(sha256sum "$home/.ssh/authorized_keys" | cut -d' ' -f1)
    fixture=absent
    test ! -f "$home/soda-acceptance-state.txt" || fixture=$(sha256sum "$home/soda-acceptance-state.txt" | cut -d' ' -f1)
    jq -cn --arg user "$user" --arg home "$home" --arg shadow "$shadow" --arg keys "$keys" --arg fixture "$fixture" '{user:$user,home:$home,shadow:$shadow,authorized_keys:$keys,fixture:$fixture}'
  done | jq -sc 'sort_by(.user)'
)
workspaces=$(
  getent passwd | awk -F: '$5 ~ /^soda-workspace=/ {print $1":"$5":"$6}' | while IFS=: read -r user marker home; do
    project=${marker##*/}
    checkout=$home/Projects/$project
    test -d "$checkout/.git"
    remote=$(runuser --user "$user" -- git -C "$checkout" remote get-url origin)
    status=$(runuser --user "$user" -- git -C "$checkout" status --porcelain=v1 --untracked-files=all | LC_ALL=C sort | jq -Rsc 'split("\n") | map(select(length > 0))')
    jq -cn --arg user "$user" --arg marker "$marker" --arg remote "$remote" --argjson status "$status" '{user:$user,marker:$marker,remote:$remote,status:$status}'
  done | jq -sc 'sort_by(.user)'
)
catalog=$(jq -S . /var/lib/soda/catalog/projects.json)
forgejo_users=$(sqlite3 /var/lib/forgejo/data/forgejo.db 'select lower_name || ":" || is_admin || ":" || is_active from user order by lower_name;' | jq -Rsc 'split("\n") | map(select(length > 0))')
tailscale=$(tailscale status --json | jq -c '.Self | {id:.ID,dns_name:.DNSName,addresses:(.TailscaleIPs|sort)}')
network=$(nmcli --terse --fields NAME,TYPE,ZONE connection show --active | LC_ALL=C sort | jq -Rsc 'split("\n") | map(select(length > 0))')
host_keys=$(sha256sum /etc/ssh/ssh_host_*_key.pub | LC_ALL=C sort | jq -Rsc 'split("\n") | map(select(length > 0))')
jq -cn --argjson accounts "$accounts" --argjson groups "$groups" --argjson homes "$homes" \
  --argjson workspaces "$workspaces" --argjson catalog "$catalog" --argjson forgejo_users "$forgejo_users" \
  --argjson tailscale "$tailscale" --argjson network "$network" --argjson host_keys "$host_keys" \
  '{accounts:$accounts,groups:$groups,homes:$homes,workspaces:$workspaces,catalog:$catalog,forgejo_users:$forgejo_users,tailscale:$tailscale,network:$network,ssh_host_keys:$host_keys}'
`

const localAccessCheck = `set -euo pipefail
test "$(systemctl is-active sshd)" = active
test "$(systemctl is-active cockpit.socket)" = active
test "$(systemctl is-active forgejo)" = active
printf 'local-network-services=pass\n'
`

const tailscaleAccessCheck = `set -euo pipefail
tailscale status --json | jq -e '.BackendState == "Running" and (.Self.Expired != true)' >/dev/null
printf 'tailscale-access=pass\n'
`
