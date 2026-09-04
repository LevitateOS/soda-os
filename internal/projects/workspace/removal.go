package workspace

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
)

// DeletionPreflight supplies the native facts required before a workspace may
// be destructively removed.
type DeletionPreflight interface {
	PasswordStatus(context.Context, linuxhost.Account) (linuxhost.PasswordStatus, error)
	PreflightDeleteAccount(context.Context, linuxhost.Account) error
}

// DeletionHost performs the already-preflighted native account deletion. The
// Linux implementation revalidates the same account evidence before mutation.
type DeletionHost interface {
	DeletionPreflight
	DeleteAccount(context.Context, linuxhost.Account) error
}

// AccountInventory supplies both exact account lookup and complete native
// workspace-account candidates. Linux remains authoritative for both facts.
type AccountInventory interface {
	AccountLookup
	CandidateAccounts(context.Context, string, string) ([]linuxhost.Account, error)
}

// Remover owns removal of one primary human's derived workspace. Catalog
// lock composition remains with the root Projects package.
type Remover struct {
	inventory AccountInventory
	deletion  DeletionHost
}

func NewRemover(inventory AccountInventory, deletion DeletionHost) Remover {
	return Remover{inventory: inventory, deletion: deletion}
}

// Remove removes only the workspace derived from primary and projectID. A retry
// after successful deletion is intentionally idempotent.
func (remover Remover) Remove(ctx context.Context, primary linuxhost.Account, projectID string) error {
	if err := catalog.ValidateID(projectID); err != nil {
		return err
	}
	uidMin, err := remover.inventory.UIDMin()
	if err != nil {
		return err
	}
	username, err := DerivedUsername(primary.Username, projectID)
	if err != nil {
		return err
	}
	account, err := remover.inventory.LookupAccount(ctx, username)
	if errors.Is(err, linuxhost.ErrAccountNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	association := Association{PrimaryUsername: primary.Username, ProjectID: projectID}
	if err = PreflightDeletion(ctx, remover.deletion, account, association, uidMin); err != nil {
		return fmt.Errorf("workspace %s was not removed; catalog state, other local workspaces, and canonical repository were not modified: %w", username, err)
	}
	if err = remover.deletion.DeleteAccount(ctx, account); err != nil {
		return fmt.Errorf("workspace %s deletion did not complete; catalog state, other local workspaces, and canonical repository were not modified: delete workspace: %w", username, err)
	}
	return nil
}

// RemoveProjectWorkspaces proves every project workspace is safe to remove
// before deleting any of them, then removes them in deterministic order. The
// caller retains ownership of catalog removal after this succeeds.
func (remover Remover) RemoveProjectWorkspaces(ctx context.Context, entry catalog.Entry, uidMin int) ([]string, error) {
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	candidates, err := remover.inventory.CandidateAccounts(ctx, Group, MarkerPrefix)
	if err != nil {
		return nil, err
	}
	targets, err := projectDeletionTargets(ctx, remover.deletion, candidates, entry.ID, uidMin)
	if err != nil {
		return nil, fmt.Errorf("no local workspaces were removed; all local workspaces, shared catalog entry, and canonical repository remain: %w", err)
	}
	removed := make([]string, 0, len(targets))
	for index, account := range targets {
		if err = remover.deletion.DeleteAccount(ctx, account); err != nil {
			remaining := accountNames(targets[index:])
			return removed, fmt.Errorf("%s; local workspaces %s, shared catalog entry, and canonical repository remain: delete workspace %s: %w", removedProjectWorkspaceDescription(removed), strings.Join(remaining, ", "), account.Username, err)
		}
		removed = append(removed, account.Username)
	}
	return removed, nil
}

// PreflightDeletion proves the workspace association, locked password, and
// native account-deletion preconditions before destructive work begins.
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

// projectDeletionTargets selects, fully preflights, and deterministically sorts
// every local workspace for one project before any destructive work begins.
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

func accountNames(accounts []linuxhost.Account) []string {
	names := make([]string, 0, len(accounts))
	for _, account := range accounts {
		names = append(names, account.Username)
	}
	return names
}

func removedProjectWorkspaceDescription(workspaces []string) string {
	if len(workspaces) == 0 {
		return "no local workspaces were removed"
	}
	return "removed local workspaces " + strings.Join(workspaces, ", ")
}
