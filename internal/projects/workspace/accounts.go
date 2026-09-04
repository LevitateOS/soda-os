package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
)

// AccountLookup supplies only the Linux account facts used by workspace
// association and creation.
type AccountLookup interface {
	UIDMin() (int, error)
	LookupAccount(context.Context, string) (linuxhost.Account, error)
}

// PasswordReader supplies the native password fact used to prove that a
// workspace cannot be entered with its own password.
type PasswordReader interface {
	PasswordStatus(context.Context, linuxhost.Account) (linuxhost.PasswordStatus, error)
}

// AuthorizedKeys supplies the descriptor-safe Linux key operations used for
// the one-time inbound-key copy.
type AuthorizedKeys interface {
	ReadAuthorizedKeys(linuxhost.Account) ([]byte, error)
	InstallAuthorizedKeys(linuxhost.Account, []byte) error
}

// OutboundKeyGenerator is the repository operation needed by preparation.
type OutboundKeyGenerator interface {
	GenerateOutboundKey(context.Context, linuxhost.Account) (string, error)
}

// RepositoryPublication is the repository operation needed to finish setup.
type RepositoryPublication interface {
	CloneExists(linuxhost.Account, catalog.Entry) (bool, error)
	Publish(context.Context, linuxhost.Account, catalog.Entry) error
}

// Accounts owns the workspace-account association, creation, and inbound-key
// copy. Its Linux dependencies remain explicit and native.
type Accounts struct {
	lookup    AccountLookup
	passwords PasswordReader
	keys      AuthorizedKeys
	runner    linuxhost.CommandRunner
}

func NewAccounts(lookup AccountLookup, passwords PasswordReader, keys AuthorizedKeys, runner linuxhost.CommandRunner) Accounts {
	return Accounts{lookup: lookup, passwords: passwords, keys: keys, runner: runner}
}

// Association reports whether the derived Linux account exists and exactly
// represents the supplied human-project association.
func (accounts Accounts) Association(ctx context.Context, primary linuxhost.Account, entry catalog.Entry) (string, bool, error) {
	if err := entry.Validate(); err != nil {
		return "", false, err
	}
	uidMin, err := accounts.lookup.UIDMin()
	if err != nil {
		return "", false, err
	}
	username, err := DerivedUsername(primary.Username, entry.ID)
	if err != nil {
		return "", false, err
	}
	account, err := accounts.lookup.LookupAccount(ctx, username)
	if errors.Is(err, linuxhost.ErrAccountNotFound) {
		return username, false, nil
	}
	if err != nil {
		return "", false, err
	}
	if err = ValidateAccount(account, primary.Username, entry.ID, uidMin); err != nil {
		return "", false, err
	}
	return username, true, nil
}

type Preparation struct {
	Username  string
	PublicKey string
}

// Prepare establishes the derived Linux account, copies the primary account's
// inbound keys once, and returns the workspace-owned outbound Git key. Native
// facts remain after a later repository authorization failure so setup can be
// retried.
func (accounts Accounts) Prepare(ctx context.Context, repository OutboundKeyGenerator, primary linuxhost.Account, entry catalog.Entry) (Preparation, error) {
	target, err := accounts.target(primary, entry)
	if err != nil {
		return Preparation{}, err
	}
	workspace, found, err := accounts.existing(ctx, target)
	if err != nil {
		return Preparation{}, err
	}
	if !found {
		workspace, err = accounts.createPrepared(ctx, target)
		if err != nil {
			return Preparation{}, err
		}
	} else if _, err = accounts.keys.ReadAuthorizedKeys(workspace); err != nil {
		return Preparation{}, fmt.Errorf("workspace %s exists without usable inbound SSH keys; restore its authorized_keys file or remove the workspace and retry setup: %w", workspace.Username, err)
	}
	publicKey, err := repository.GenerateOutboundKey(ctx, workspace)
	if err != nil {
		return Preparation{}, fmt.Errorf("workspace %s and its inbound SSH keys were retained; outbound Git key generation can be retried: %w", workspace.Username, err)
	}
	return Preparation{Username: workspace.Username, PublicKey: publicKey}, nil
}

// Publish completes setup for an already prepared workspace through one native
// SSH clone. A complete clone is the only successful outcome.
func (accounts Accounts) Publish(ctx context.Context, repository RepositoryPublication, primary linuxhost.Account, entry catalog.Entry) (string, error) {
	target, err := accounts.target(primary, entry)
	if err != nil {
		return "", err
	}
	workspace, found, err := accounts.existing(ctx, target)
	if err != nil {
		return "", err
	}
	if !found {
		return "", errors.New("workspace preparation is required before cloning")
	}
	exists, err := repository.CloneExists(workspace, entry)
	if err != nil {
		return "", err
	}
	if !exists {
		if err = repository.Publish(ctx, workspace, entry); err != nil {
			return "", fmt.Errorf("workspace %s, its SSH keys, and outbound Git key were retained; clone can be retried: %w", workspace.Username, err)
		}
	}
	return workspace.Username, nil
}

type accountTarget struct {
	primary linuxhost.Account
	entry   catalog.Entry
	uidMin  int
}

func (accounts Accounts) target(primary linuxhost.Account, entry catalog.Entry) (accountTarget, error) {
	if err := entry.Validate(); err != nil {
		return accountTarget{}, err
	}
	uidMin, err := accounts.lookup.UIDMin()
	if err != nil {
		return accountTarget{}, err
	}
	if _, err = DerivedUsername(primary.Username, entry.ID); err != nil {
		return accountTarget{}, err
	}
	return accountTarget{primary: primary, entry: entry, uidMin: uidMin}, nil
}

func (accounts Accounts) existing(ctx context.Context, target accountTarget) (linuxhost.Account, bool, error) {
	username, _ := DerivedUsername(target.primary.Username, target.entry.ID)
	workspace, err := accounts.lookup.LookupAccount(ctx, username)
	if errors.Is(err, linuxhost.ErrAccountNotFound) {
		return linuxhost.Account{}, false, nil
	}
	if err != nil {
		return linuxhost.Account{}, false, err
	}
	if err = accounts.validate(ctx, workspace, target); err != nil {
		return linuxhost.Account{}, false, err
	}
	return workspace, true, nil
}

func (accounts Accounts) createPrepared(ctx context.Context, target accountTarget) (linuxhost.Account, error) {
	keys, err := accounts.keys.ReadAuthorizedKeys(target.primary)
	if err != nil {
		return linuxhost.Account{}, err
	}
	workspace, err := accounts.create(ctx, target)
	if err != nil {
		return linuxhost.Account{}, err
	}
	if err = accounts.keys.InstallAuthorizedKeys(workspace, keys); err != nil {
		return linuxhost.Account{}, fmt.Errorf("workspace %s was retained because inbound SSH keys may be incomplete: %w", workspace.Username, err)
	}
	return workspace, nil
}

func (accounts Accounts) create(ctx context.Context, target accountTarget) (linuxhost.Account, error) {
	username, _ := DerivedUsername(target.primary.Username, target.entry.ID)
	marker, _ := Marker(target.primary.Username, target.entry.ID)
	result, err := accounts.runner.Run(ctx, linuxhost.Command{Name: "/usr/sbin/useradd", Args: []string{
		"--create-home",
		"--user-group",
		"--groups", Group,
		"--shell", Shell,
		"--home-dir", "/home/" + username,
		"--comment", marker,
		"--", username,
	}})
	if err != nil {
		return linuxhost.Account{}, fmt.Errorf("create workspace account %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return linuxhost.Account{}, fmt.Errorf("create workspace account %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	workspace, err := accounts.lookup.LookupAccount(ctx, username)
	if err != nil {
		return linuxhost.Account{}, fmt.Errorf("verify created workspace account %s: %w", username, err)
	}
	if err = accounts.validate(ctx, workspace, target); err != nil {
		return linuxhost.Account{}, fmt.Errorf("new workspace was retained because its Linux state is invalid: %w", err)
	}
	return workspace, nil
}

func (accounts Accounts) validate(ctx context.Context, workspace linuxhost.Account, target accountTarget) error {
	if err := ValidateAccount(workspace, target.primary.Username, target.entry.ID, target.uidMin); err != nil {
		return err
	}
	status, err := accounts.passwords.PasswordStatus(ctx, workspace)
	if err != nil {
		return err
	}
	if status != linuxhost.PasswordLocked {
		return fmt.Errorf("workspace account %s does not have a locked password", workspace.Username)
	}
	return nil
}
