package runners

import (
	"context"
	"errors"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
)

type Authorizer interface {
	RequireAdministrator(context.Context, linuxhost.PKExecIdentity) error
}

type LinuxAccounts interface {
	UIDMin() (int, error)
	LookupAccount(context.Context, string) (linuxhost.Account, error)
}

type LinuxAuthorizer struct {
	Accounts LinuxAccounts
}

func (authorizer LinuxAuthorizer) RequireAdministrator(ctx context.Context, actor linuxhost.PKExecIdentity) error {
	uidMin, err := authorizer.Accounts.UIDMin()
	if err != nil {
		return err
	}
	account, err := authorizer.Accounts.LookupAccount(ctx, actor.Username)
	if err != nil {
		return err
	}
	if account.Username != actor.Username || account.UID != actor.UID {
		return errors.New("Linux account identity changed before runner authorization")
	}
	if account.UID < uidMin || !account.HasInteractiveShell() || !account.IsAdministrator() {
		return errors.New("administrator status is required")
	}
	return nil
}
