// Package people composes primary Linux-account authorization and Soda-aware
// human deletion without owning a person database or mirroring Linux state.
package people

import (
	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
)

func IsPrimary(account linuxhost.Account, uidMin int) bool {
	return account.Username != "" && account.UID >= uidMin &&
		account.HasInteractiveShell() && !account.HasGroup(workspace.Group)
}

func IsAdministrator(account linuxhost.Account, uidMin int) bool {
	return IsPrimary(account, uidMin) && account.IsAdministrator()
}
