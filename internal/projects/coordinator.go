package projects

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/LevitateOS/soda-os/internal/projects/people"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
	"github.com/LevitateOS/soda-os/internal/strictjson"
)

// Coordinator composes the unprivileged Projects request with the concrete
// catalog, workspace facts, lock primitives, and fixed pkexec helper.
type Coordinator struct {
	store          *catalog.Store
	authorizer     Authorizer
	workspaces     workspace.Accounts
	setupLocks     workspace.SetupLocker
	operationLocks OperationLocker
	privileged     PKExecInvoker
}

func NewSystemCoordinator(host *linuxhost.Native) Coordinator {
	return Coordinator{
		store:          catalog.NewSystemStore(),
		authorizer:     NewAuthorizer(host),
		workspaces:     workspace.NewAccounts(host, host, host, host),
		setupLocks:     workspace.NewSetupLocker("/run/user"),
		operationLocks: NewSystemOperationLocker(),
		privileged:     NewSystemPKExecInvoker(),
	}
}

func (coordinator Coordinator) Execute(ctx context.Context, actorUsername, action string, input io.Reader) (any, error) {
	if coordinator.store == nil {
		return nil, errors.New("Projects coordinator was not constructed")
	}
	primary, uidMin, err := coordinator.authorizer.Primary(ctx, actorUsername)
	if err != nil {
		return nil, err
	}
	return coordinator.dispatch(ctx, primary, uidMin, action, input)
}

func (coordinator Coordinator) dispatch(ctx context.Context, primary linuxhost.Account, uidMin int, action string, input io.Reader) (any, error) {
	switch action {
	case "list":
		return coordinator.executeList(ctx, primary, uidMin, input)
	case "add-existing":
		return coordinator.executeAddExisting(ctx, input)
	case "edit":
		return coordinator.executeEdit(ctx, input)
	case "setup":
		return coordinator.executeSetup(ctx, primary, input)
	case "remove-workspace":
		return coordinator.executeRemoveWorkspace(ctx, input)
	case "remove":
		return coordinator.executeRemove(ctx, input)
	case "delete-human":
		return coordinator.executeDeleteHuman(ctx, input)
	default:
		return nil, fmt.Errorf("unsupported soda-projects action %q", action)
	}
}

func (coordinator Coordinator) executeRemoveWorkspace(ctx context.Context, input io.Reader) (SuccessResponse, error) {
	var request ProjectRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return SuccessResponse{}, err
	}
	if err := coordinator.privileged.WorkspaceRemove(ctx, request); err != nil {
		return SuccessResponse{}, err
	}
	return SuccessResponse{OK: true}, nil
}

func (coordinator Coordinator) executeList(ctx context.Context, primary linuxhost.Account, uidMin int, input io.Reader) (ListResponse, error) {
	var request EmptyRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return ListResponse{}, err
	}
	return coordinator.list(ctx, primary, uidMin)
}

func (coordinator Coordinator) executeAddExisting(ctx context.Context, input io.Reader) (ProjectMutationResponse, error) {
	var request AddExistingRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return ProjectMutationResponse{}, err
	}
	if err := request.Validate(); err != nil {
		return ProjectMutationResponse{}, err
	}
	return coordinator.privileged.CatalogAdd(ctx, request)
}

func (coordinator Coordinator) executeEdit(ctx context.Context, input io.Reader) (ProjectMutationResponse, error) {
	var request EditRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return ProjectMutationResponse{}, err
	}
	if err := request.Validate(); err != nil {
		return ProjectMutationResponse{}, err
	}
	return coordinator.privileged.CatalogEdit(ctx, request)
}

func (coordinator Coordinator) executeSetup(ctx context.Context, primary linuxhost.Account, input io.Reader) (SetupResponse, error) {
	var request SetupRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return SetupResponse{}, err
	}
	return coordinator.setup(ctx, primary, request)
}

func (coordinator Coordinator) executeRemove(ctx context.Context, input io.Reader) (SuccessResponse, error) {
	var request ProjectRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return SuccessResponse{}, err
	}
	if err := coordinator.privileged.ProjectRemove(ctx, request); err != nil {
		return SuccessResponse{}, err
	}
	return SuccessResponse{OK: true}, nil
}

func (coordinator Coordinator) executeDeleteHuman(ctx context.Context, input io.Reader) (SuccessResponse, error) {
	var request DeleteHumanRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return SuccessResponse{}, err
	}
	if err := coordinator.privileged.HumanDelete(ctx, HelperHumanRequest(request)); err != nil {
		return SuccessResponse{}, err
	}
	return SuccessResponse{OK: true}, nil
}

func (coordinator Coordinator) list(ctx context.Context, primary linuxhost.Account, uidMin int) (ListResponse, error) {
	entries, err := coordinator.store.List()
	if err != nil {
		return ListResponse{}, err
	}
	views := make([]ProjectView, 0, len(entries))
	for _, entry := range entries {
		view, viewErr := projectView(ctx, coordinator.workspaces, primary, entry)
		if viewErr != nil {
			return ListResponse{}, viewErr
		}
		views = append(views, view)
	}
	return ListResponse{
		Projects:    views,
		CurrentUser: CurrentUserView{Username: primary.Username, Administrator: people.IsAdministrator(primary, uidMin)},
	}, nil
}

func projectView(ctx context.Context, accounts workspace.Accounts, primary linuxhost.Account, entry catalog.Entry) (ProjectView, error) {
	username, exists, err := accounts.Association(ctx, primary, entry)
	if err != nil {
		return ProjectView{}, err
	}
	return newProjectView(entry, username, exists), nil
}
