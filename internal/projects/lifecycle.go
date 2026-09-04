package projects

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Lifecycle struct {
	Catalog  *Catalog
	Platform Platform
}

func (lifecycle Lifecycle) AuthorizePrimary(ctx context.Context, username string) (Account, int, error) {
	uidMin, err := lifecycle.Platform.UIDMin()
	if err != nil {
		return Account{}, 0, err
	}
	account, err := lifecycle.Platform.LookupAccount(ctx, username)
	if err != nil {
		return Account{}, 0, err
	}
	if !account.IsPrimary(uidMin) {
		return Account{}, 0, errors.New("caller is not a supported primary Linux account")
	}
	return account, uidMin, nil
}

func (lifecycle Lifecycle) WorkspaceAssociation(ctx context.Context, primary Account, projectID string) (string, bool, error) {
	username, err := DerivedUsername(primary.Username, projectID)
	if err != nil {
		return "", false, err
	}
	uidMin, err := lifecycle.Platform.UIDMin()
	if err != nil {
		return "", false, err
	}
	account, err := lifecycle.Platform.LookupAccount(ctx, username)
	if errors.Is(err, ErrAccountNotFound) {
		return username, false, nil
	}
	if err != nil {
		return "", false, err
	}
	if err = account.ValidateWorkspace(primary.Username, projectID, uidMin); err != nil {
		return "", false, err
	}
	return username, true, nil
}

type WorkspacePreparation struct {
	Username  string
	PublicKey string
}

func (lifecycle Lifecycle) PrepareWorkspace(ctx context.Context, primaryUsername string, request HelperWorkspaceRequest) (WorkspacePreparation, error) {
	lock, err := lifecycle.Platform.WorkspaceOperationSharedLock()
	if err != nil {
		return WorkspacePreparation{}, fmt.Errorf("lock workspace operations: %w", err)
	}
	var preparation WorkspacePreparation
	operationErr := lifecycle.Catalog.Exclusive(func() error {
		var prepareErr error
		preparation, prepareErr = lifecycle.prepareWorkspaceUnlocked(ctx, primaryUsername, request)
		return prepareErr
	})
	if err = closeLockWithError(lock, operationErr, "workspace operations"); err != nil {
		return WorkspacePreparation{}, err
	}
	return preparation, nil
}

func (lifecycle Lifecycle) prepareWorkspaceUnlocked(ctx context.Context, primaryUsername string, request HelperWorkspaceRequest) (WorkspacePreparation, error) {
	target, err := lifecycle.prepareWorkspaceTarget(ctx, primaryUsername, request)
	if err != nil {
		return WorkspacePreparation{}, err
	}
	workspace, found, err := lifecycle.existingWorkspace(ctx, target)
	if err != nil {
		return WorkspacePreparation{}, err
	}
	if !found {
		workspace, err = lifecycle.createPreparedWorkspace(ctx, target)
		if err != nil {
			return WorkspacePreparation{}, err
		}
	}
	return lifecycle.workspacePreparation(ctx, workspace)
}

func (lifecycle Lifecycle) createPreparedWorkspace(ctx context.Context, target publishTarget) (Account, error) {
	keys, err := lifecycle.Platform.ReadAuthorizedKeys(target.primary)
	if err != nil {
		return Account{}, err
	}
	workspace, err := lifecycle.Platform.CreateWorkspace(ctx, target.primary, target.entry.ID)
	if err != nil {
		return Account{}, err
	}
	if err = lifecycle.validateWorkspace(ctx, workspace, target); err != nil {
		return Account{}, fmt.Errorf("new workspace was retained because its Linux state is invalid: %w", err)
	}
	if err = lifecycle.Platform.InstallAuthorizedKeys(workspace, keys); err != nil {
		return Account{}, fmt.Errorf("workspace %s was retained because inbound SSH keys may be incomplete: %w", workspace.Username, err)
	}
	return workspace, nil
}

func (lifecycle Lifecycle) workspacePreparation(ctx context.Context, workspace Account) (WorkspacePreparation, error) {
	publicKey, err := lifecycle.Platform.GenerateWorkspaceGitKey(ctx, workspace)
	if err != nil {
		return WorkspacePreparation{}, fmt.Errorf("workspace %s and its inbound SSH keys were retained; outbound Git key generation can be retried: %w", workspace.Username, err)
	}
	return WorkspacePreparation{Username: workspace.Username, PublicKey: publicKey}, nil
}

type publishTarget struct {
	primary        Account
	entry          CatalogEntry
	uidMin         int
	workspaceTools []string
	projectTools   []string
}

func (lifecycle Lifecycle) prepareWorkspaceTarget(ctx context.Context, primaryUsername string, request HelperWorkspaceRequest) (publishTarget, error) {
	primary, uidMin, err := lifecycle.AuthorizePrimary(ctx, primaryUsername)
	if err != nil {
		return publishTarget{}, err
	}
	entry, err := lifecycle.Catalog.Get(request.ID)
	if err != nil {
		return publishTarget{}, err
	}
	if request.CanonicalURL != entry.CanonicalURL {
		return publishTarget{}, errors.New("project URL changed after clone; run setup again")
	}
	if err = ValidateToolSelections(request.WorkspaceTools); err != nil {
		return publishTarget{}, err
	}
	if err = ValidateToolSelections(request.ProjectTools); err != nil {
		return publishTarget{}, err
	}
	return publishTarget{
		primary: primary, entry: entry, uidMin: uidMin,
		workspaceTools: request.WorkspaceTools, projectTools: request.ProjectTools,
	}, nil
}

func (lifecycle Lifecycle) existingWorkspace(ctx context.Context, target publishTarget) (Account, bool, error) {
	username, _ := DerivedUsername(target.primary.Username, target.entry.ID)
	existing, err := lifecycle.Platform.LookupAccount(ctx, username)
	if errors.Is(err, ErrAccountNotFound) {
		return Account{}, false, nil
	}
	if err != nil {
		return Account{}, false, err
	}
	if err = lifecycle.validateWorkspace(ctx, existing, target); err != nil {
		return Account{}, false, err
	}
	return existing, true, nil
}

func (lifecycle Lifecycle) CompleteWorkspace(ctx context.Context, primaryUsername string, request HelperWorkspaceRequest) (string, error) {
	lock, err := lifecycle.Platform.WorkspaceOperationSharedLock()
	if err != nil {
		return "", fmt.Errorf("lock workspace operations: %w", err)
	}
	var username string
	operationErr := lifecycle.Catalog.Exclusive(func() error {
		var completeErr error
		username, completeErr = lifecycle.completeWorkspaceUnlocked(ctx, primaryUsername, request)
		return completeErr
	})
	return username, closeLockWithError(lock, operationErr, "workspace operations")
}

func (lifecycle Lifecycle) completeWorkspaceUnlocked(ctx context.Context, primaryUsername string, request HelperWorkspaceRequest) (string, error) {
	target, err := lifecycle.prepareWorkspaceTarget(ctx, primaryUsername, request)
	if err != nil {
		return "", err
	}
	workspace, found, err := lifecycle.existingWorkspace(ctx, target)
	if err != nil {
		return "", err
	}
	if !found {
		return "", errors.New("workspace preparation is required before cloning")
	}
	ready, err := lifecycle.Platform.WorkspaceReady(workspace, target.entry.ID)
	if err != nil {
		return "", err
	}
	if !ready {
		if err = lifecycle.Platform.CloneWorkspace(ctx, workspace, target.entry.ID, target.entry.CanonicalURL); err != nil {
			return "", fmt.Errorf("workspace %s, its SSH keys, and outbound Git key were retained; clone can be retried: %w", workspace.Username, err)
		}
	}
	if err = lifecycle.Platform.InstallMiseTools(ctx, workspace, target.entry.ID, target.workspaceTools, target.projectTools); err != nil {
		return "", fmt.Errorf("workspace %s and its complete clone were retained; mise setup can be retried: %w", workspace.Username, err)
	}
	return workspace.Username, nil
}

func (lifecycle Lifecycle) validateWorkspace(ctx context.Context, workspace Account, target publishTarget) error {
	if err := workspace.ValidateWorkspace(target.primary.Username, target.entry.ID, target.uidMin); err != nil {
		return err
	}
	return lifecycle.Platform.ValidatePasswordLocked(ctx, workspace)
}

func (lifecycle Lifecycle) InstallTools(ctx context.Context, actorUsername string, request HelperToolRequest) error {
	if request.Scope != "workspace" && request.Scope != "project" {
		return errors.New("mise tool scope must be workspace or project")
	}
	if len(request.Tools) == 0 {
		return errors.New("at least one mise tool is required")
	}
	if err := ValidateToolSelections(request.Tools); err != nil {
		return err
	}
	lock, err := lifecycle.Platform.WorkspaceOperationSharedLock()
	if err != nil {
		return fmt.Errorf("lock workspace operations: %w", err)
	}
	operationErr := lifecycle.installToolsLocked(ctx, actorUsername, request)
	return closeLockWithError(lock, operationErr, "workspace operations")
}

func (lifecycle Lifecycle) installToolsLocked(ctx context.Context, actorUsername string, request HelperToolRequest) error {
	primary, uidMin, err := lifecycle.AuthorizePrimary(ctx, actorUsername)
	if err != nil {
		return err
	}
	if _, err = lifecycle.Catalog.Get(request.ID); err != nil {
		return err
	}
	username, _ := DerivedUsername(primary.Username, request.ID)
	workspace, err := lifecycle.Platform.LookupAccount(ctx, username)
	if errors.Is(err, ErrAccountNotFound) {
		return errors.New("set up your workspace before installing tools")
	}
	if err != nil {
		return err
	}
	if err = workspace.ValidateWorkspace(primary.Username, request.ID, uidMin); err != nil {
		return err
	}
	if err = lifecycle.Platform.ValidatePasswordLocked(ctx, workspace); err != nil {
		return err
	}
	ready, err := lifecycle.Platform.WorkspaceReady(workspace, request.ID)
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("workspace does not contain a complete clone")
	}
	workspaceTools, projectTools := request.Tools, []string(nil)
	if request.Scope == "project" {
		workspaceTools, projectTools = nil, request.Tools
	}
	if err = lifecycle.Platform.InstallMiseTools(ctx, workspace, request.ID, workspaceTools, projectTools); err != nil {
		return fmt.Errorf("workspace %s was retained; mise installation can be retried: %w", workspace.Username, err)
	}
	return nil
}

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
	workspace, err := lifecycle.Platform.LookupAccount(ctx, username)
	if errors.Is(err, ErrAccountNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = lifecycle.preflightWorkspaceDeletion(ctx, workspace, primary.Username, projectID, uidMin); err != nil {
		return fmt.Errorf("no workspace was removed; workspace %s, shared catalog entry, other local workspaces, and canonical repository remain: %w", username, err)
	}
	if err = lifecycle.Platform.DeleteAccount(ctx, workspace); err != nil {
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
	if !actor.IsAdministrator(uidMin) {
		return errors.New("administrator status is required")
	}
	if _, err := lifecycle.Catalog.Get(projectID); err != nil {
		return err
	}
	accounts, err := lifecycle.Platform.WorkspaceAccounts(ctx)
	if err != nil {
		return err
	}
	targets, err := lifecycle.projectDeletionTargets(ctx, accounts, projectID, uidMin)
	if err != nil {
		return fmt.Errorf("no local workspaces were removed; all local workspaces, shared catalog entry, and canonical repository remain: %w", err)
	}
	removed := make([]string, 0, len(targets))
	for index, account := range targets {
		if err = lifecycle.Platform.DeleteAccount(ctx, account); err != nil {
			remaining := accountNames(targets[index:])
			return fmt.Errorf("%s; local workspaces %s, shared catalog entry, and canonical repository remain: delete workspace %s: %w", removedProjectWorkspaceDescription(removed), strings.Join(remaining, ", "), account.Username, err)
		}
		removed = append(removed, account.Username)
	}
	if err = lifecycle.Platform.RemoveMiseProject(projectID); err != nil {
		return fmt.Errorf("%s; shared mise project storage, shared catalog entry, and canonical repository remain: %w", removedProjectWorkspaceDescription(removed), err)
	}
	if err = lifecycle.Catalog.removeUnlocked(projectID); err != nil {
		return fmt.Errorf("%s and shared mise project storage; shared catalog entry and canonical repository remain: %w", removedProjectWorkspaceDescription(removed), err)
	}
	return nil
}

func (lifecycle Lifecycle) projectDeletionTargets(ctx context.Context, accounts []Account, projectID string, uidMin int) ([]Account, error) {
	targets := []Account{}
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

func accountNames(accounts []Account) []string {
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

func closeLockWithError(lock io.Closer, operationErr error, description string) error {
	closeErr := lock.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("unlock %s: %w", description, closeErr)
	}
	return errors.Join(operationErr, closeErr)
}

func (lifecycle Lifecycle) deleteHumanUnlocked(ctx context.Context, actorUsername, targetUsername string) error {
	target, uidMin, err := lifecycle.authorizeHumanDeletion(ctx, actorUsername, targetUsername)
	if err != nil {
		return err
	}
	accounts, err := lifecycle.Platform.WorkspaceAccounts(ctx)
	if err != nil {
		return err
	}
	if err = lifecycle.Platform.PreflightDeleteAccount(ctx, target); err != nil {
		return err
	}
	workspaces, err := lifecycle.humanDeletionTargets(ctx, accounts, targetUsername, uidMin)
	if err != nil {
		return err
	}
	removed := make([]string, 0, len(workspaces))
	for _, account := range workspaces {
		if err = lifecycle.Platform.DeleteAccount(ctx, account); err != nil {
			return fmt.Errorf("%s; workspace %s, Forgejo account, and primary Linux account remain: delete workspace: %w", removedWorkspaceDescription(removed), account.Username, err)
		}
		removed = append(removed, account.Username)
	}
	if err = lifecycle.Platform.DeleteForgejoUser(ctx, target.Username); err != nil && !errors.Is(err, ErrForgejoUserNotFound) {
		return fmt.Errorf("%s; Forgejo account and primary Linux account %s remain: delete Forgejo account: %w", removedWorkspaceDescription(removed), target.Username, err)
	}
	if err = lifecycle.Platform.DeleteAccount(ctx, target); err != nil {
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

func (lifecycle Lifecycle) authorizeHumanDeletion(ctx context.Context, actorUsername, targetUsername string) (Account, int, error) {
	actor, uidMin, err := lifecycle.AuthorizePrimary(ctx, actorUsername)
	if err != nil {
		return Account{}, 0, err
	}
	if !actor.IsAdministrator(uidMin) {
		return Account{}, 0, errors.New("administrator status is required")
	}
	target, err := lifecycle.Platform.LookupAccount(ctx, targetUsername)
	if err != nil {
		return Account{}, 0, err
	}
	if !target.IsPrimary(uidMin) {
		return Account{}, 0, errors.New("target is not a supported primary Linux account")
	}
	return target, uidMin, nil
}

func (lifecycle Lifecycle) humanDeletionTargets(ctx context.Context, accounts []Account, targetUsername string, uidMin int) ([]Account, error) {
	targets := []Account{}
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

func (lifecycle Lifecycle) preflightWorkspaceDeletion(
	ctx context.Context,
	account Account,
	primary string,
	projectID string,
	uidMin int,
) error {
	if err := account.ValidateWorkspace(primary, projectID, uidMin); err != nil {
		return err
	}
	if err := lifecycle.Platform.ValidatePasswordLocked(ctx, account); err != nil {
		return err
	}
	return lifecycle.Platform.PreflightDeleteAccount(ctx, account)
}
