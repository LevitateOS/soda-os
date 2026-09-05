# Arguments are explicit primary/workspace/project triples, not a host inventory.
set -euo pipefail
shopt -s inherit_errexit
export LC_ALL=C
test "$#" -gt 0
test "$(( $# % 3 ))" -eq 0
while test "$#" -gt 0; do
  primary=$1
  workspace=$2
  project=$3
  shift 3
  printf 'primary=%s workspace=%s project=%s\n' "$primary" "$workspace" "$project"
  for user in "$primary" "$workspace"; do
    entry=$(getent passwd "$user")
    printf '%s\n' "$entry"
    home=$(cut -d: -f6 <<<"$entry")
    test -d "$home"
    id -Gn "$user" | tr ' ' '\n' | sort
    getent shadow "$user" | sha256sum
    stat -c '%n %u:%g:%a' "$home" "$home/.ssh" "$home/.ssh/authorized_keys"
    sha256sum "$home/.ssh/authorized_keys"
  done
  # The last account above is the workspace; Linux supplied its actual home.
  checkout="$home/Projects/$project"
  test -d "$checkout/.git"
  stat -c '%n %u:%g:%a' "$checkout" "$home/.ssh/id_ed25519_soda"
  sha256sum "$home/.ssh/id_ed25519_soda" "$home/.ssh/id_ed25519_soda.pub" "$home/soda-acceptance-state.txt"
  runuser --user "$workspace" -- git -C "$checkout" fsck --no-reflogs >&2
  runuser --user "$workspace" -- git -C "$checkout" rev-parse HEAD
  runuser --user "$workspace" -- git -C "$checkout" remote get-url origin
  runuser --user "$workspace" -- git -C "$checkout" ls-remote origin | sort
  # Observe contents AND metadata of committed, modified and untracked files.
  find "$checkout" -path "$checkout/.git" -prune -o -printf '%P %U:%G:%m %l\n' | sort
  find "$checkout" -path "$checkout/.git" -prune -o -type f -print0 | sort -z | xargs -0 -r sha256sum
  runuser --user "$workspace" -- git -C "$checkout" status --porcelain=v1 --untracked-files=all | sort
  node="$home/.local/share/mise/installs/node/22.14.0/bin/node"
  stat -c '%n %u:%g:%a' "$node"
  sha256sum "$node"
  runuser --user "$workspace" -- /bin/bash -c 'cd "$1"; mise exec -- node --version' preservation "$checkout"
  # Download into a fresh, explicitly owned probe: unchanged refs alone do not
  # establish that the canonical Git objects still exist and are readable.
  origin=$(runuser --user "$workspace" -- git -C "$checkout" remote get-url origin)
  probe=$(runuser --user "$workspace" -- mktemp -d "$home/.acceptance-git.XXXXXX")
  trap 'rm -rf -- "$probe"' EXIT
  runuser --user "$workspace" -- git clone --quiet --no-checkout -- "$origin" "$probe"
  runuser --user "$workspace" -- git -C "$probe" fsck --no-reflogs >&2
  runuser --user "$workspace" -- git -C "$probe" rev-parse HEAD
  rm -rf -- "$probe"
  trap - EXIT
done
jq -Sc . /var/lib/soda/catalog/projects.json
tailscale status --json | jq -ec '.Self | {id:.ID,dns_name:.DNSName,addresses:(.TailscaleIPs|sort)}'
nmcli --terse --fields NAME,TYPE,ZONE connection show --active | sort
sha256sum /etc/ssh/ssh_host_*_key.pub | sort
