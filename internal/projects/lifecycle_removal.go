package projects

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
)

func (lifecycle Lifecycle) RemoveWorkspace(ctx context.Context, actorUsername, projectID string) error {
	lock, err := lifecycle.Platform.WorkspaceOperationExclusiveLock()
	if err != nil {
		return fmt.Errorf("lock workspace operations: %w", err)
	}
	operationErr := lifecycle.Catalog.Exclusive(func() error {
		return lifecycle.removeWorkspaceUnlocked(ctx, actorUsername, projectID)
	})
	return closeLockWithError(lock, operationErr, "workspace operations")
}

func (lifecycle Lifecycle) removeWorkspaceUnlocked(ctx context.Context, actorUsername, projectID string) error {
	primary, uidMin, err := lifecycle.AuthorizePrimary(ctx, actorUsername)
	if err != nil {
		return err
	}
	if _, err = lifecycle.Catalog.Get(projectID); err != nil {
		return err
	}
	username, _ := DerivedUsername(primary.Username, projectID)
	workspace, err := lifecycle.Host.LookupAccount(ctx, username)
	if errors.Is(err, linuxhost.ErrAccountNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = lifecycle.preflightWorkspaceDeletion(ctx, workspace, primary.Username, projectID, uidMin); err != nil {
		return fmt.Errorf("no workspace was removed; workspace %s, shared catalog entry, other local workspaces, and canonical repository remain: %w", username, err)
	}
	if err = lifecycle.Host.DeleteAccount(ctx, workspace); err != nil {
		return fmt.Errorf("no workspace was removed; workspace %s, shared catalog entry, other local workspaces, and canonical repository remain: delete workspace: %w", username, err)
	}
	return nil
}

func (lifecycle Lifecycle) RemoveProject(ctx context.Context, actorUsername, projectID string) error {
	lock, err := lifecycle.Platform.WorkspaceOperationExclusiveLock()
	if err != nil {
		return fmt.Errorf("lock workspace operations: %w", err)
	}
	operationErr := lifecycle.Catalog.Exclusive(func() error {
		return lifecycle.removeProjectUnlocked(ctx, actorUsername, projectID)
	})
	return closeLockWithError(lock, operationErr, "workspace operations")
}

func (lifecycle Lifecycle) removeProjectUnlocked(ctx context.Context, actorUsername, projectID string) error {
	actor, uidMin, err := lifecycle.AuthorizePrimary(ctx, actorUsername)
	if err != nil {
		return err
	}
	if !isAdministrator(actor, uidMin) {
		return errors.New("administrator status is required")
	}
	if _, err := lifecycle.Catalog.Get(projectID); err != nil {
		return err
	}
	accounts, err := lifecycle.Host.CandidateAccounts(ctx, WorkspaceGroup, workspaceMarkerKey)
	if err != nil {
		return err
	}
	targets, err := lifecycle.projectDeletionTargets(ctx, accounts, projectID, uidMin)
	if err != nil {
		return fmt.Errorf("no local workspaces were removed; all local workspaces, shared catalog entry, and canonical repository remain: %w", err)
	}
	removed := make([]string, 0, len(targets))
	for index, account := range targets {
		if err = lifecycle.Host.DeleteAccount(ctx, account); err != nil {
			remaining := accountNames(targets[index:])
			return fmt.Errorf("%s; local workspaces %s, shared catalog entry, and canonical repository remain: delete workspace %s: %w", removedProjectWorkspaceDescription(removed), strings.Join(remaining, ", "), account.Username, err)
		}
		removed = append(removed, account.Username)
	}
	if err = lifecycle.Catalog.removeUnlocked(projectID); err != nil {
		return fmt.Errorf("%s; shared catalog entry and canonical repository remain: %w", removedProjectWorkspaceDescription(removed), err)
	}
	return nil
}

func (lifecycle Lifecycle) projectDeletionTargets(ctx context.Context, accounts []linuxhost.Account, projectID string, uidMin int) ([]linuxhost.Account, error) {
	targets := []linuxhost.Account{}
	for _, account := range accounts {
		primary, associatedProject, err := ParseWorkspaceMarker(account.GECOS)
		if err != nil {
			return nil, err
		}
		if associatedProject != projectID {
			continue
		}
		if err = lifecycle.preflightWorkspaceDeletion(ctx, account, primary, associatedProject, uidMin); err != nil {
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

func (lifecycle Lifecycle) DeleteHuman(ctx context.Context, actorUsername, targetUsername string) error {
	lock, err := lifecycle.Platform.WorkspaceOperationExclusiveLock()
	if err != nil {
		return fmt.Errorf("lock workspace operations: %w", err)
	}
	operationErr := lifecycle.Catalog.Exclusive(func() error {
		return lifecycle.deleteHumanUnlocked(ctx, actorUsername, targetUsername)
	})
	return closeLockWithError(lock, operationErr, "workspace operations")
}

func (lifecycle Lifecycle) deleteHumanUnlocked(ctx context.Context, actorUsername, targetUsername string) error {
	target, uidMin, err := lifecycle.authorizeHumanDeletion(ctx, actorUsername, targetUsername)
	if err != nil {
		return err
	}
	accounts, err := lifecycle.Host.CandidateAccounts(ctx, WorkspaceGroup, workspaceMarkerKey)
	if err != nil {
		return err
	}
	if err = lifecycle.Host.PreflightDeleteAccount(ctx, target); err != nil {
		return err
	}
	workspaces, err := lifecycle.humanDeletionTargets(ctx, accounts, targetUsername, uidMin)
	if err != nil {
		return err
	}
	removed := make([]string, 0, len(workspaces))
	for _, account := range workspaces {
		if err = lifecycle.Host.DeleteAccount(ctx, account); err != nil {
			return fmt.Errorf("%s; workspace %s, Forgejo account, and primary Linux account remain: delete workspace: %w", removedWorkspaceDescription(removed), account.Username, err)
		}
		removed = append(removed, account.Username)
	}
	if err = lifecycle.Platform.DeleteForgejoUser(ctx, target.Username); err != nil && !errors.Is(err, ErrForgejoUserNotFound) {
		return fmt.Errorf("%s; Forgejo account and primary Linux account %s remain: delete Forgejo account: %w", removedWorkspaceDescription(removed), target.Username, err)
	}
	if err = lifecycle.Host.DeleteAccount(ctx, target); err != nil {
		return fmt.Errorf("%s and Forgejo account %s; primary Linux account remains: %w", removedWorkspaceDescription(removed), target.Username, err)
	}
	return nil
}

func removedWorkspaceDescription(workspaces []string) string {
	if len(workspaces) == 0 {
		return "no Soda workspaces were removed"
	}
	return "removed Soda workspaces " + strings.Join(workspaces, ", ")
}

func (lifecycle Lifecycle) authorizeHumanDeletion(ctx context.Context, actorUsername, targetUsername string) (linuxhost.Account, int, error) {
	actor, uidMin, err := lifecycle.AuthorizePrimary(ctx, actorUsername)
	if err != nil {
		return linuxhost.Account{}, 0, err
	}
	if !isAdministrator(actor, uidMin) {
		return linuxhost.Account{}, 0, errors.New("administrator status is required")
	}
	target, err := lifecycle.Host.LookupAccount(ctx, targetUsername)
	if err != nil {
		return linuxhost.Account{}, 0, err
	}
	if !isPrimaryAccount(target, uidMin) {
		return linuxhost.Account{}, 0, errors.New("target is not a supported primary Linux account")
	}
	return target, uidMin, nil
}

func (lifecycle Lifecycle) humanDeletionTargets(ctx context.Context, accounts []linuxhost.Account, targetUsername string, uidMin int) ([]linuxhost.Account, error) {
	targets := []linuxhost.Account{}
	for _, account := range accounts {
		primary, projectID, err := ParseWorkspaceMarker(account.GECOS)
		if err != nil {
			return nil, err
		}
		if primary != targetUsername {
			continue
		}
		if err = lifecycle.preflightWorkspaceDeletion(ctx, account, primary, projectID, uidMin); err != nil {
			return nil, err
		}
		targets = append(targets, account)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Username < targets[j].Username })
	return targets, nil
}

func (lifecycle Lifecycle) preflightWorkspaceDeletion(ctx context.Context, account linuxhost.Account, primary, projectID string, uidMin int) error {
	if err := validateWorkspaceAccount(account, primary, projectID, uidMin); err != nil {
		return err
	}
	status, err := lifecycle.Host.PasswordStatus(ctx, account)
	if err != nil {
		return err
	}
	if status != linuxhost.PasswordLocked {
		return fmt.Errorf("workspace account %s does not have a locked password", account.Username)
	}
	return lifecycle.Host.PreflightDeleteAccount(ctx, account)
}
