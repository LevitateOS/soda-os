package projects

import (
	"context"
	"errors"
	"fmt"
	"io"
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

func (lifecycle Lifecycle) Publish(ctx context.Context, primaryUsername string, request HelperWorkspaceRequest) (string, error) {
	lock, err := lifecycle.Platform.WorkspaceOperationSharedLock()
	if err != nil {
		return "", fmt.Errorf("lock workspace operations: %w", err)
	}
	var username string
	operationErr := lifecycle.Catalog.Exclusive(func() error {
		var operationErr error
		username, operationErr = lifecycle.publishUnlocked(ctx, primaryUsername, request)
		return operationErr
	})
	return username, closeLockWithError(lock, operationErr, "workspace operations")
}

func (lifecycle Lifecycle) publishUnlocked(ctx context.Context, primaryUsername string, request HelperWorkspaceRequest) (string, error) {
	target, err := lifecycle.preparePublish(ctx, primaryUsername, request)
	if err != nil {
		return "", err
	}
	username, found, err := lifecycle.existingWorkspace(ctx, target)
	if err != nil {
		return "", err
	}
	if found {
		return username, nil
	}
	return lifecycle.createAndPublishWorkspace(ctx, target)
}

type publishTarget struct {
	primary Account
	entry   CatalogEntry
	uidMin  int
}

func (lifecycle Lifecycle) preparePublish(ctx context.Context, primaryUsername string, request HelperWorkspaceRequest) (publishTarget, error) {
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
	return publishTarget{primary: primary, entry: entry, uidMin: uidMin}, nil
}

func (lifecycle Lifecycle) existingWorkspace(ctx context.Context, target publishTarget) (string, bool, error) {
	username, _ := DerivedUsername(target.primary.Username, target.entry.ID)
	existing, err := lifecycle.Platform.LookupAccount(ctx, username)
	if errors.Is(err, ErrAccountNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if err = lifecycle.validateWorkspace(ctx, existing, target); err != nil {
		return "", false, err
	}
	ready, err := lifecycle.Platform.WorkspaceReady(existing, target.entry.ID)
	if err != nil {
		return "", false, err
	}
	if !ready {
		return "", false, errors.New("workspace account exists without a complete clone; explicit operator cleanup is required")
	}
	return username, true, nil
}

func (lifecycle Lifecycle) createAndPublishWorkspace(ctx context.Context, target publishTarget) (string, error) {
	keys, err := lifecycle.Platform.ReadAuthorizedKeys(target.primary)
	if err != nil {
		return "", err
	}
	workspace, err := lifecycle.Platform.CreateWorkspace(ctx, target.primary, target.entry.ID)
	if err != nil {
		return "", err
	}
	if err = lifecycle.validateWorkspace(ctx, workspace, target); err != nil {
		return "", fmt.Errorf("new workspace was retained because its Linux state is invalid: %w", err)
	}
	if err = lifecycle.Platform.PublishWorkspace(ctx, target.primary, workspace, target.entry.ID); err != nil {
		if safetyErr := lifecycle.Platform.SafeToRemoveIncomplete(workspace, target.entry.ID); safetyErr != nil {
			return "", errors.Join(err, fmt.Errorf("incomplete workspace was retained: %w", safetyErr))
		}
		return "", lifecycle.deleteNewWorkspace(ctx, workspace, err)
	}
	if err = lifecycle.Platform.InstallWorkspaceTea(target.primary, workspace); err != nil {
		if safetyErr := lifecycle.Platform.SafeToRemoveIncomplete(workspace, target.entry.ID); safetyErr != nil {
			return "", errors.Join(err, fmt.Errorf("incomplete workspace was retained: %w", safetyErr))
		}
		return "", lifecycle.deleteNewWorkspace(ctx, workspace, err)
	}
	if err = lifecycle.Platform.InstallAuthorizedKeys(workspace, keys); err != nil {
		return "", lifecycle.handleKeyInstallFailure(ctx, workspace, target.entry.ID, err)
	}
	return workspace.Username, nil
}

func (lifecycle Lifecycle) handleKeyInstallFailure(ctx context.Context, workspace Account, projectID string, cause error) error {
	if errors.Is(cause, ErrAuthorizedKeysPublished) {
		return fmt.Errorf("workspace was retained because SSH keys may be active: %w", cause)
	}
	if safetyErr := lifecycle.Platform.SafeToRemoveIncomplete(workspace, projectID); safetyErr != nil {
		return errors.Join(cause, fmt.Errorf("incomplete workspace was retained: %w", safetyErr))
	}
	return lifecycle.deleteNewWorkspace(ctx, workspace, cause)
}

func (lifecycle Lifecycle) validateWorkspace(ctx context.Context, workspace Account, target publishTarget) error {
	if err := workspace.ValidateWorkspace(target.primary.Username, target.entry.ID, target.uidMin); err != nil {
		return err
	}
	return lifecycle.Platform.ValidatePasswordLocked(ctx, workspace)
}

func (lifecycle Lifecycle) deleteNewWorkspace(ctx context.Context, workspace Account, cause error) error {
	deleteErr := lifecycle.Platform.DeleteAccount(context.WithoutCancel(ctx), workspace)
	return errors.Join(cause, deleteErr)
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
	if _, _, err := lifecycle.AuthorizePrimary(ctx, actorUsername); err != nil {
		return err
	}
	if _, err := lifecycle.Catalog.Get(projectID); err != nil {
		return err
	}
	accounts, err := lifecycle.Platform.WorkspaceAccounts(ctx)
	if err != nil {
		return err
	}
	uidMin, err := lifecycle.Platform.UIDMin()
	if err != nil {
		return err
	}
	targets, err := lifecycle.projectDeletionTargets(ctx, accounts, projectID, uidMin)
	if err != nil {
		return err
	}
	for _, account := range targets {
		if err = lifecycle.Platform.DeleteAccount(ctx, account); err != nil {
			return err
		}
	}
	return lifecycle.Catalog.removeUnlocked(projectID)
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
	return targets, nil
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
	for _, account := range workspaces {
		if err = lifecycle.Platform.DeleteAccount(ctx, account); err != nil {
			return fmt.Errorf("delete workspace %s: %w", account.Username, err)
		}
	}
	return lifecycle.Platform.DeleteAccount(ctx, target)
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
