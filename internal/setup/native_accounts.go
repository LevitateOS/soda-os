package setup

import (
	"context"
	"sort"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
)

const workspaceGroup = "soda-workspaces"

type accountsHost interface {
	UIDMin() (int, error)
	LookupGroup(context.Context, string) (linuxhost.Group, error)
	LookupAccount(context.Context, string) (linuxhost.Account, error)
}

type NativeAccounts struct {
	Host accountsHost
}

func (accounts NativeAccounts) Administrators(ctx context.Context) ([]Administrator, error) {
	host := accounts.Host
	if host == nil {
		host = linuxhost.NewNative()
	}
	wheel, err := host.LookupGroup(ctx, linuxhost.AdministratorGroup)
	if err != nil {
		return nil, err
	}
	members := make([]string, 0, len(wheel.Members))
	for member := range wheel.Members {
		members = append(members, member)
	}
	sort.Strings(members)
	uidMin, err := host.UIDMin()
	if err != nil {
		return nil, err
	}
	administrators := make([]Administrator, 0, len(members))
	for _, username := range members {
		account, lookupErr := host.LookupAccount(ctx, username)
		if lookupErr != nil || !isAdministrator(account, uidMin) {
			continue
		}
		administrators = append(administrators, Administrator{
			Username: username,
		})
	}
	return administrators, nil
}

func isAdministrator(account linuxhost.Account, uidMin int) bool {
	return account.Username != "" && account.UID >= uidMin && account.HasInteractiveShell() &&
		!account.HasGroup(workspaceGroup) && account.HasGroup(linuxhost.AdministratorGroup)
}
