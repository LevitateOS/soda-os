package projects

import (
	"context"
	"errors"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/people"
)

// AccountLookup is the narrow Linux fact boundary used to authorize the
// current primary account. Linux remains authoritative for both identity and
// administrator membership.
type AccountLookup interface {
	UIDMin() (int, error)
	LookupAccount(context.Context, string) (linuxhost.Account, error)
}

type Authorizer struct {
	accounts AccountLookup
}

func NewAuthorizer(accounts AccountLookup) Authorizer {
	return Authorizer{accounts: accounts}
}

func (authorizer Authorizer) Primary(ctx context.Context, username string) (linuxhost.Account, int, error) {
	if authorizer.accounts == nil {
		return linuxhost.Account{}, 0, errors.New("Projects authorizer was not constructed")
	}
	uidMin, err := authorizer.accounts.UIDMin()
	if err != nil {
		return linuxhost.Account{}, 0, err
	}
	account, err := authorizer.accounts.LookupAccount(ctx, username)
	if err != nil {
		return linuxhost.Account{}, 0, err
	}
	if !people.IsPrimary(account, uidMin) {
		return linuxhost.Account{}, 0, errors.New("caller is not a supported primary Linux account")
	}
	return account, uidMin, nil
}
