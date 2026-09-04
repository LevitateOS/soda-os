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
	workspace, err := lifecycle.completeWorkspaceForTools(ctx, actorUsername, request.ID)
	if err != nil {
		return err
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

func (lifecycle Lifecycle) completeWorkspaceForTools(ctx context.Context, actorUsername, projectID string) (Account, error) {
	primary, uidMin, err := lifecycle.AuthorizePrimary(ctx, actorUsername)
	if err != nil {
		return Account{}, err
	}
	if _, err = lifecycle.Catalog.Get(projectID); err != nil {
		return Account{}, err
	}
	username, _ := DerivedUsername(primary.Username, projectID)
	workspace, err := lifecycle.Platform.LookupAccount(ctx, username)
	if errors.Is(err, ErrAccountNotFound) {
		return Account{}, errors.New("set up your workspace before installing tools")
	}
	if err != nil {
		return Account{}, err
	}
	if err = workspace.ValidateWorkspace(primary.Username, projectID, uidMin); err != nil {
		return Account{}, err
	}
	if err = lifecycle.Platform.ValidatePasswordLocked(ctx, workspace); err != nil {
		return Account{}, err
	}
	ready, err := lifecycle.Platform.WorkspaceReady(workspace, projectID)
	if err != nil {
		return Account{}, err
	}
	if !ready {
		return Account{}, errors.New("workspace does not contain a complete clone")
	}
	return workspace, nil
}

func closeLockWithError(lock io.Closer, operationErr error, description string) error {
	closeErr := lock.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("unlock %s: %w", description, closeErr)
	}
	return errors.Join(operationErr, closeErr)
}
