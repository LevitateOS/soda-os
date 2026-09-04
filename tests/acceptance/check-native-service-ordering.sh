#!/bin/sh
# Run on an installed candidate, after native cloud-init provisioning finishes.
set -eu

systemd-analyze verify --man=no multi-user.target cloud-init.target forgejo.service
test "$(systemctl show forgejo-init.service --property=Type --value)" = oneshot
test "$(systemctl show forgejo-init.service --property=RemainAfterExit --value)" = no
test "$(systemctl show forgejo-init.service --property=ActiveState --value)" = inactive
for unit in forgejo.service forgejo-init.service; do
    if systemctl show "$unit" --property=After --value | tr ' ' '\n' |
        grep -Ex 'cloud-final.service|cloud-init.target'; then
        echo "Forgejo must not wait for cloud-init finalization" >&2
        exit 1
    fi
done
if journalctl --boot --no-pager --unit=init.scope |
    grep -Ei 'ordering cycle|deleted.*job.*break.*cycle|discarded.*job'; then
    echo "Boot contains service-ordering failures" >&2
    exit 1
fi
