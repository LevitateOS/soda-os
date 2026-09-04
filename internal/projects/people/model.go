// Package people composes primary Linux-account authorization and Soda-aware
// human deletion without owning a person database or mirroring Linux state.
package people

import (
	"errors"
	"regexp"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
)

var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return errors.New("username must match [a-z][a-z0-9-]{0,23}")
	}
	return nil
}

func IsPrimary(account linuxhost.Account, uidMin int) bool {
	return account.UID >= uidMin && ValidateUsername(account.Username) == nil &&
		account.HasInteractiveShell() && !account.HasGroup(workspace.Group)
}

func IsAdministrator(account linuxhost.Account, uidMin int) bool {
	return IsPrimary(account, uidMin) && account.IsAdministrator()
}
