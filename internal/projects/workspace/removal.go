package workspace

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
)

// DeletionPreflight supplies read-only native deletion checks.
type DeletionPreflight interface {
	PasswordStatus(context.Context, linuxhost.Account) (linuxhost.PasswordStatus, error)
	PreflightDeleteAccount(context.Context, linuxhost.Account) error
}

type AccountInventory interface {
	AccountLookup
	CandidateAccounts(context.Context, string, string) ([]linuxhost.Account, error)
}

// Remover selects and preflights exact workspace accounts. The Projects helper
// owns confirmation, lock composition, execution receipts, and catalog removal.
type Remover struct {
	inventory AccountInventory
	deletion  DeletionPreflight
}

func NewRemover(inventory AccountInventory, deletion DeletionPreflight) Remover {
	return Remover{inventory: inventory, deletion: deletion}
}

func (remover Remover) Targets(ctx context.Context, primary linuxhost.Account, projectID string) ([]linuxhost.Account, error) {
	if err := catalog.ValidateID(projectID); err != nil {
		return nil, err
	}
	uidMin, err := remover.inventory.UIDMin()
	if err != nil {
		return nil, err
	}
	username, err := DerivedUsername(primary.Username, projectID)
	if err != nil {
		return nil, err
	}
	account, err := remover.inventory.LookupAccount(ctx, username)
	if errors.Is(err, linuxhost.ErrAccountNotFound) {
		return []linuxhost.Account{}, nil
	}
	if err != nil {
		return nil, err
	}
	association := Association{PrimaryUsername: primary.Username, ProjectID: projectID}
	if err = PreflightDeletion(ctx, remover.deletion, account, association, uidMin); err != nil {
		return nil, fmt.Errorf("workspace %s cannot be removed: %w", username, err)
	}
	return []linuxhost.Account{account}, nil
}

func (remover Remover) ProjectTargets(ctx context.Context, projectID string, uidMin int) ([]linuxhost.Account, error) {
	candidates, err := remover.inventory.CandidateAccounts(ctx, Group, MarkerPrefix)
	if err != nil {
		return nil, err
	}
	return projectDeletionTargets(ctx, remover.deletion, candidates, projectID, uidMin)
}

func PreflightDeletion(ctx context.Context, host DeletionPreflight, account linuxhost.Account, association Association, uidMin int) error {
	if err := ValidateAccount(account, association.PrimaryUsername, association.ProjectID, uidMin); err != nil {
		return err
	}
	status, err := host.PasswordStatus(ctx, account)
	if err != nil {
		return err
	}
	if status != linuxhost.PasswordLocked {
		return fmt.Errorf("workspace account %s does not have a locked password", account.Username)
	}
	return host.PreflightDeleteAccount(ctx, account)
}

func projectDeletionTargets(ctx context.Context, host DeletionPreflight, accounts []linuxhost.Account, projectID string, uidMin int) ([]linuxhost.Account, error) {
	if err := validateProjectID(projectID); err != nil {
		return nil, err
	}
	targets := []linuxhost.Account{}
	for _, account := range accounts {
		primaryUsername, associatedProject, err := ParseMarker(account.GECOS)
		if err != nil {
			return nil, err
		}
		if associatedProject != projectID {
			continue
		}
		association := Association{PrimaryUsername: primaryUsername, ProjectID: associatedProject}
		if err = PreflightDeletion(ctx, host, account, association, uidMin); err != nil {
			return nil, err
		}
		targets = append(targets, account)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Username < targets[j].Username })
	return targets, nil
}
