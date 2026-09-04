package projects

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/LevitateOS/soda-os/internal/projects/people"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
	"github.com/LevitateOS/soda-os/internal/strictjson"
)

// Helper composes the fixed privileged actions from concrete domain owners.
// It has no generic lifecycle or platform surface.
type Helper struct {
	store          *catalog.Store
	authorizer     Authorizer
	workspaces     workspace.Accounts
	repository     workspace.Repository
	remover        workspace.Remover
	people         people.Deletion
	operationLocks OperationLocker
}

func NewSystemHelper(host *linuxhost.Native) Helper {
	return Helper{
		store:          catalog.NewSystemStore(),
		authorizer:     NewAuthorizer(host),
		workspaces:     workspace.NewAccounts(host, host, host, host),
		repository:     workspace.NewRepository(host, host),
		remover:        workspace.NewRemover(host, host),
		people:         people.Deletion{Host: host, Forgejo: people.Forgejo{Runner: host}},
		operationLocks: NewSystemOperationLocker(),
	}
}

func (helper Helper) Execute(ctx context.Context, actor linuxhost.PKExecIdentity, action string, input io.Reader) (any, error) {
	if helper.store == nil {
		return nil, errors.New("Projects helper was not constructed")
	}
	if _, _, err := helper.authorizeActor(ctx, actor); err != nil {
		return nil, err
	}
	return helper.dispatch(ctx, actor, action, input)
}

func (helper Helper) authorizeActor(ctx context.Context, actor linuxhost.PKExecIdentity) (linuxhost.Account, int, error) {
	account, uidMin, err := helper.authorizer.Primary(ctx, actor.Username)
	if err != nil {
		return linuxhost.Account{}, 0, err
	}
	if account.UID != actor.UID {
		return linuxhost.Account{}, 0, errors.New("PKEXEC_UID no longer matches the authorized Linux account")
	}
	return account, uidMin, nil
}

func (helper Helper) dispatch(ctx context.Context, actor linuxhost.PKExecIdentity, action string, input io.Reader) (any, error) {
	switch action {
	case "catalog-add":
		return helper.catalogAdd(ctx, actor, input)
	case "catalog-edit":
		return helper.catalogEdit(ctx, actor, input)
	case "workspace-prepare":
		return helper.workspacePrepare(ctx, actor, input)
	case "workspace-publish":
		return helper.workspacePublish(ctx, actor, input)
	case "workspace-remove":
		return helper.workspaceRemove(ctx, actor, input)
	case "project-remove":
		return helper.projectRemove(ctx, actor, input)
	case "human-delete":
		return helper.humanDelete(ctx, actor, input)
	default:
		return nil, fmt.Errorf("unsupported workspace helper action %q", action)
	}
}

func (helper Helper) catalogAdd(ctx context.Context, actor linuxhost.PKExecIdentity, input io.Reader) (ProjectMutationResponse, error) {
	var request HelperCatalogRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return ProjectMutationResponse{}, err
	}
	locked, err := helper.store.Lock()
	if err != nil {
		return ProjectMutationResponse{}, err
	}
	primary, _, operationErr := helper.authorizeActor(ctx, actor)
	var view ProjectView
	if operationErr == nil {
		view, operationErr = projectView(ctx, helper.workspaces, primary, request)
	}
	if operationErr == nil {
		operationErr = locked.Add(request)
	}
	if operationErr = errors.Join(operationErr, locked.Close()); operationErr != nil {
		return ProjectMutationResponse{}, operationErr
	}
	return ProjectMutationResponse{OK: true, Project: view}, nil
}

func (helper Helper) catalogEdit(ctx context.Context, actor linuxhost.PKExecIdentity, input io.Reader) (ProjectMutationResponse, error) {
	var request HelperEditRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return ProjectMutationResponse{}, err
	}
	locked, err := helper.store.Lock()
	if err != nil {
		return ProjectMutationResponse{}, err
	}
	primary, _, operationErr := helper.authorizeActor(ctx, actor)
	var updated catalog.Entry
	if operationErr == nil {
		updated, operationErr = locked.Get(request.ID)
	}
	var view ProjectView
	if operationErr == nil {
		view, operationErr = projectView(ctx, helper.workspaces, primary, request.Apply(updated))
	}
	if operationErr == nil {
		updated, operationErr = locked.Edit(request)
		view = newProjectView(updated, view.WorkspaceUsername, view.WorkspaceExists)
	}
	if operationErr = errors.Join(operationErr, locked.Close()); operationErr != nil {
		return ProjectMutationResponse{}, operationErr
	}
	return ProjectMutationResponse{OK: true, Project: view}, nil
}

func (helper Helper) workspacePrepare(ctx context.Context, actor linuxhost.PKExecIdentity, input io.Reader) (WorkspacePreparationResponse, error) {
	var request HelperWorkspaceRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return WorkspacePreparationResponse{}, err
	}
	lock, err := helper.operationLocks.Shared()
	if err != nil {
		return WorkspacePreparationResponse{}, fmt.Errorf("lock workspace operations: %w", err)
	}
	primary, _, err := helper.authorizeActor(ctx, actor)
	if err != nil {
		return WorkspacePreparationResponse{}, closeLockWithError(lock, err, "workspace operations")
	}
	entry, err := helper.workspaceEntry(request)
	if err != nil {
		return WorkspacePreparationResponse{}, closeLockWithError(lock, err, "workspace operations")
	}
	prepared, err := helper.workspaces.Prepare(ctx, helper.repository, primary, entry)
	if err = closeLockWithError(lock, err, "workspace operations"); err != nil {
		return WorkspacePreparationResponse{}, err
	}
	return WorkspacePreparationResponse{
		OK: true, WorkspaceUsername: prepared.Username, WorkspacePublicKey: prepared.PublicKey,
	}, nil
}

func (helper Helper) workspacePublish(ctx context.Context, actor linuxhost.PKExecIdentity, input io.Reader) (WorkspacePublicationResponse, error) {
	var request HelperWorkspaceRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return WorkspacePublicationResponse{}, err
	}
	lock, err := helper.operationLocks.Shared()
	if err != nil {
		return WorkspacePublicationResponse{}, fmt.Errorf("lock workspace operations: %w", err)
	}
	primary, _, err := helper.authorizeActor(ctx, actor)
	if err != nil {
		return WorkspacePublicationResponse{}, closeLockWithError(lock, err, "workspace operations")
	}
	entry, err := helper.workspaceEntry(request)
	if err != nil {
		return WorkspacePublicationResponse{}, closeLockWithError(lock, err, "workspace operations")
	}
	username, err := helper.workspaces.Publish(ctx, helper.repository, primary, entry)
	if err = closeLockWithError(lock, err, "workspace operations"); err != nil {
		return WorkspacePublicationResponse{}, err
	}
	return WorkspacePublicationResponse{OK: true, WorkspaceUsername: username}, nil
}

func (helper Helper) workspaceEntry(request HelperWorkspaceRequest) (catalog.Entry, error) {
	entry, err := helper.store.Get(request.ID)
	if err != nil {
		return catalog.Entry{}, err
	}
	if request.CanonicalURL != entry.CanonicalURL {
		return catalog.Entry{}, errors.New("project URL changed after clone; run setup again")
	}
	return entry, nil
}

func (helper Helper) workspaceRemove(ctx context.Context, actor linuxhost.PKExecIdentity, input io.Reader) (SuccessResponse, error) {
	var request ProjectRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return SuccessResponse{}, err
	}
	lock, err := helper.operationLocks.Exclusive()
	if err != nil {
		return SuccessResponse{}, fmt.Errorf("lock workspace operations: %w", err)
	}
	primary, _, err := helper.authorizeActor(ctx, actor)
	if err == nil {
		err = helper.remover.Remove(ctx, primary, request.ID)
	}
	if err = closeLockWithError(lock, err, "workspace operations"); err != nil {
		return SuccessResponse{}, err
	}
	return SuccessResponse{OK: true}, nil
}

func (helper Helper) projectRemove(ctx context.Context, actor linuxhost.PKExecIdentity, input io.Reader) (SuccessResponse, error) {
	var request ProjectRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return SuccessResponse{}, err
	}
	lock, err := helper.operationLocks.Exclusive()
	if err != nil {
		return SuccessResponse{}, fmt.Errorf("lock workspace operations: %w", err)
	}
	operationErr := helper.removeProjectLocked(ctx, actor, request.ID)
	if err = closeLockWithError(lock, operationErr, "workspace operations"); err != nil {
		return SuccessResponse{}, err
	}
	return SuccessResponse{OK: true}, nil
}

func (helper Helper) removeProjectLocked(ctx context.Context, identity linuxhost.PKExecIdentity, projectID string) error {
	locked, err := helper.store.Lock()
	if err != nil {
		return err
	}
	actor, uidMin, operationErr := helper.authorizeActor(ctx, identity)
	if operationErr == nil && !people.IsAdministrator(actor, uidMin) {
		operationErr = errors.New("administrator status is required")
	}
	if operationErr == nil {
		operationErr = helper.removeProjectWithCatalogLock(ctx, locked, projectID, uidMin)
	}
	return errors.Join(operationErr, locked.Close())
}

func (helper Helper) removeProjectWithCatalogLock(ctx context.Context, locked *catalog.LockedStore, projectID string, uidMin int) error {
	entry, err := locked.Get(projectID)
	if err != nil {
		return err
	}
	removed, err := helper.remover.RemoveProjectWorkspaces(ctx, entry, uidMin)
	if err != nil {
		return err
	}
	if err = locked.Remove(projectID); err != nil {
		return fmt.Errorf("%s; shared catalog entry and canonical repository remain: %w", removedProjectWorkspaceDescription(removed), err)
	}
	return nil
}

func removedProjectWorkspaceDescription(workspaces []string) string {
	if len(workspaces) == 0 {
		return "no local workspaces were removed"
	}
	return "removed local workspaces " + strings.Join(workspaces, ", ")
}

func (helper Helper) humanDelete(ctx context.Context, identity linuxhost.PKExecIdentity, input io.Reader) (SuccessResponse, error) {
	var request HelperHumanRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return SuccessResponse{}, err
	}
	lock, err := helper.operationLocks.Exclusive()
	if err != nil {
		return SuccessResponse{}, fmt.Errorf("lock workspace operations: %w", err)
	}
	actor, uidMin, operationErr := helper.authorizeActor(ctx, identity)
	if operationErr == nil {
		operationErr = helper.people.Delete(ctx, actor, uidMin, request.Username)
	}
	if err = closeLockWithError(lock, operationErr, "workspace operations"); err != nil {
		return SuccessResponse{}, err
	}
	return SuccessResponse{OK: true}, nil
}
