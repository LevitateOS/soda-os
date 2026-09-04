package projects

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/LevitateOS/soda-os/internal/tailnet"
)

type EndpointSource interface {
	Endpoints(context.Context) (forgejoURL, sshHost string, err error)
}

type TailnetEndpoints struct{}

func (TailnetEndpoints) Endpoints(ctx context.Context) (string, string, error) {
	identity, err := tailnet.New(tailnet.Options{}).Identity(ctx)
	if err != nil {
		return "", "", err
	}
	return "http://" + identity + ":30000", identity, nil
}

type PrivilegedProjects interface {
	CatalogAdd(context.Context, HelperCatalogRequest) error
	CatalogEdit(context.Context, HelperCatalogRequest) error
	WorkspacePublish(context.Context, HelperWorkspaceRequest) (MutationResponse, error)
	ProjectRemove(context.Context, ProjectRequest) error
	HumanDelete(context.Context, HelperHumanRequest) error
	HumanCreate(context.Context, HelperHumanCreateRequest) error
	HumanPublish(context.Context, HelperHumanPublishRequest) error
}

type PKExecInvoker struct {
	Binary     string
	HelperPath string
}

func (invoker PKExecInvoker) CatalogAdd(ctx context.Context, request HelperCatalogRequest) error {
	_, err := invoker.mutation(ctx, "catalog-add", request)
	return err
}

func (invoker PKExecInvoker) CatalogEdit(ctx context.Context, request HelperCatalogRequest) error {
	_, err := invoker.mutation(ctx, "catalog-edit", request)
	return err
}

func (invoker PKExecInvoker) WorkspacePublish(ctx context.Context, request HelperWorkspaceRequest) (MutationResponse, error) {
	return invoker.mutation(ctx, "workspace-publish", request)
}

func (invoker PKExecInvoker) ProjectRemove(ctx context.Context, request ProjectRequest) error {
	_, err := invoker.mutation(ctx, "project-remove", request)
	return err
}

func (invoker PKExecInvoker) HumanDelete(ctx context.Context, request HelperHumanRequest) error {
	_, err := invoker.mutation(ctx, "human-delete", request)
	return err
}

func (invoker PKExecInvoker) HumanCreate(ctx context.Context, request HelperHumanCreateRequest) error {
	_, err := invoker.mutation(ctx, "human-create", request)
	return err
}

func (invoker PKExecInvoker) HumanPublish(ctx context.Context, request HelperHumanPublishRequest) error {
	_, err := invoker.mutation(ctx, "human-publish", request)
	return err
}

func (invoker PKExecInvoker) mutation(ctx context.Context, action string, request any) (MutationResponse, error) {
	var response MutationResponse
	if err := invoker.invoke(ctx, action, request, &response); err != nil {
		return MutationResponse{}, err
	}
	if !response.OK {
		return MutationResponse{}, fmt.Errorf("privileged %s did not complete", action)
	}
	return response, nil
}

func (invoker PKExecInvoker) invoke(ctx context.Context, action string, request, response any) error {
	binary := invoker.Binary
	if binary == "" {
		binary = "/usr/bin/pkexec"
	}
	helper := invoker.HelperPath
	if helper == "" {
		helper = "/usr/libexec/soda/soda-workspace-helper"
	}
	contents, err := json.Marshal(request)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, binary, "--disable-internal-agent", helper, action)
	command.Stdin = bytes.NewReader(contents)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err = command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("privileged %s: %s", action, message)
	}
	if response == nil {
		return nil
	}
	decoder := json.NewDecoder(&stdout)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(response); err != nil {
		return fmt.Errorf("decode privileged %s result: %w", action, err)
	}
	return nil
}

type Coordinator struct {
	Catalog    *Catalog
	Lifecycle  Lifecycle
	Platform   Platform
	Privileged PrivilegedProjects
	Forgejo    ForgejoClient
	Cloner     Cloner
	Endpoints  EndpointSource
}

func (coordinator Coordinator) Execute(ctx context.Context, actorUsername, action string, input io.Reader) (any, error) {
	primary, uidMin, err := coordinator.Lifecycle.AuthorizePrimary(ctx, actorUsername)
	if err != nil {
		return nil, err
	}
	switch action {
	case "list":
		return coordinator.executeList(ctx, primary, uidMin, input)
	case "add-existing":
		return coordinator.executeAddExisting(ctx, primary, input)
	case "create-forgejo":
		return coordinator.executeCreateForgejo(ctx, primary, input)
	case "edit":
		return coordinator.executeEdit(ctx, primary, input)
	case "setup":
		return coordinator.executeSetup(ctx, primary, input)
	case "remove":
		return coordinator.executeRemove(ctx, input)
	case "delete-human":
		return coordinator.executeDeleteHuman(ctx, input)
	case "add-person":
		return coordinator.executeAddPerson(ctx, primary, uidMin, input)
	default:
		return nil, fmt.Errorf("unsupported soda-projects action %q", action)
	}
}

func (coordinator Coordinator) executeAddPerson(ctx context.Context, actor Account, uidMin int, input io.Reader) (any, error) {
	var request AddPersonRequest
	if err := DecodeRequest(input, &request); err != nil {
		return nil, err
	}
	if !actor.IsAdministrator(uidMin) {
		return nil, errors.New("administrator status is required")
	}
	return coordinator.addPerson(ctx, request)
}

func (coordinator Coordinator) executeList(ctx context.Context, primary Account, uidMin int, input io.Reader) (any, error) {
	var request EmptyRequest
	if err := DecodeRequest(input, &request); err != nil {
		return nil, err
	}
	return coordinator.list(ctx, primary, uidMin)
}

func (coordinator Coordinator) executeAddExisting(ctx context.Context, primary Account, input io.Reader) (any, error) {
	var request AddExistingRequest
	if err := DecodeRequest(input, &request); err != nil {
		return nil, err
	}
	entry := CatalogEntry(request)
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	if err := coordinator.Privileged.CatalogAdd(ctx, HelperCatalogRequest(request)); err != nil {
		return nil, err
	}
	return coordinator.projectResult(ctx, primary, entry)
}

func (coordinator Coordinator) executeCreateForgejo(ctx context.Context, primary Account, input io.Reader) (any, error) {
	var request CreateForgejoRequest
	if err := DecodeRequest(input, &request); err != nil {
		return nil, err
	}
	return coordinator.createForgejo(ctx, primary, request)
}

func (coordinator Coordinator) executeEdit(ctx context.Context, primary Account, input io.Reader) (any, error) {
	var request EditRequest
	if err := DecodeRequest(input, &request); err != nil {
		return nil, err
	}
	entry := CatalogEntry{ID: request.ID, DisplayName: request.DisplayName, CanonicalURL: request.CanonicalURL}
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	if err := coordinator.Privileged.CatalogEdit(ctx, HelperCatalogRequest(request)); err != nil {
		return nil, err
	}
	return coordinator.projectResult(ctx, primary, entry)
}

func (coordinator Coordinator) executeSetup(ctx context.Context, primary Account, input io.Reader) (any, error) {
	var request SetupRequest
	if err := DecodeRequest(input, &request); err != nil {
		return nil, err
	}
	return coordinator.setup(ctx, primary, request)
}

func (coordinator Coordinator) executeRemove(ctx context.Context, input io.Reader) (any, error) {
	var request ProjectRequest
	if err := DecodeRequest(input, &request); err != nil {
		return nil, err
	}
	if err := coordinator.Privileged.ProjectRemove(ctx, request); err != nil {
		return nil, err
	}
	return MutationResponse{OK: true}, nil
}

func (coordinator Coordinator) executeDeleteHuman(ctx context.Context, input io.Reader) (any, error) {
	var request DeleteHumanRequest
	if err := DecodeRequest(input, &request); err != nil {
		return nil, err
	}
	if err := coordinator.Privileged.HumanDelete(ctx, HelperHumanRequest(request)); err != nil {
		return nil, err
	}
	return MutationResponse{OK: true}, nil
}

func (coordinator Coordinator) list(ctx context.Context, primary Account, uidMin int) (ListResponse, error) {
	entries, err := coordinator.Catalog.List()
	if err != nil {
		return ListResponse{}, err
	}
	views := make([]ProjectView, 0, len(entries))
	for _, entry := range entries {
		view, viewErr := coordinator.projectView(ctx, primary, entry)
		if viewErr != nil {
			return ListResponse{}, viewErr
		}
		views = append(views, view)
	}
	forgejoURL, sshHost, err := coordinator.Endpoints.Endpoints(ctx)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{
		Projects:    views,
		CurrentUser: CurrentUserView{Username: primary.Username, Administrator: primary.IsAdministrator(uidMin)},
		ForgejoURL:  forgejoURL,
		SSHHost:     sshHost,
	}, nil
}

func (coordinator Coordinator) createForgejo(ctx context.Context, primary Account, request CreateForgejoRequest) (MutationResponse, error) {
	entryForValidation := CatalogEntry{ID: request.ID, DisplayName: request.DisplayName, CanonicalURL: "ssh://git@example.invalid/repository"}
	if err := entryForValidation.Validate(); err != nil {
		return MutationResponse{}, err
	}
	entries, err := coordinator.Catalog.List()
	if err != nil {
		return MutationResponse{}, err
	}
	for _, entry := range entries {
		if entry.ID == request.ID {
			return MutationResponse{}, fmt.Errorf("project %q already exists", request.ID)
		}
	}
	forgejoURL, _, err := coordinator.Endpoints.Endpoints(ctx)
	if err != nil {
		return MutationResponse{}, err
	}
	created, err := coordinator.Forgejo.Create(ctx, ForgejoCreateRequest{
		BaseURL:  forgejoURL,
		Username: primary.Username,
		Password: request.Password,
		ID:       request.ID,
	})
	if err != nil {
		return MutationResponse{}, err
	}
	entry := CatalogEntry{ID: request.ID, DisplayName: request.DisplayName, CanonicalURL: created.CanonicalURL}
	if err = coordinator.Privileged.CatalogAdd(ctx, HelperCatalogRequest(entry)); err != nil {
		return MutationResponse{}, fmt.Errorf("repository was created at %s but catalog publication failed: %w", created.CanonicalURL, err)
	}
	return coordinator.projectResult(ctx, primary, entry)
}

func (coordinator Coordinator) setup(ctx context.Context, primary Account, request SetupRequest) (MutationResponse, error) {
	if !projectIDPattern.MatchString(request.ID) {
		return MutationResponse{}, errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	setupLock, err := coordinator.Platform.SetupLock(primary, request.ID)
	if err != nil {
		return MutationResponse{}, fmt.Errorf("lock project setup: %w", err)
	}
	response, setupErr := coordinator.setupWithOperationLock(ctx, primary, request)
	return response, closeLockWithError(setupLock, setupErr, "project setup")
}

func (coordinator Coordinator) setupWithOperationLock(ctx context.Context, primary Account, request SetupRequest) (MutationResponse, error) {
	lock, err := coordinator.Platform.WorkspaceOperationSharedLock()
	if err != nil {
		return MutationResponse{}, fmt.Errorf("lock workspace operations: %w", err)
	}
	response, setupErr := coordinator.setupLocked(ctx, primary, request)
	return response, closeLockWithError(lock, setupErr, "workspace operations")
}

func (coordinator Coordinator) setupLocked(ctx context.Context, primary Account, request SetupRequest) (MutationResponse, error) {
	entry, err := coordinator.Catalog.Get(request.ID)
	if err != nil {
		return MutationResponse{}, err
	}
	response, ready, err := coordinator.existingWorkspace(ctx, primary, entry)
	if err != nil {
		return MutationResponse{}, err
	}
	if ready {
		return response, nil
	}
	return coordinator.setupNewWorkspace(ctx, primary, entry, request)
}

func (coordinator Coordinator) existingWorkspace(ctx context.Context, primary Account, entry CatalogEntry) (MutationResponse, bool, error) {
	workspaceUsername, exists, err := coordinator.Lifecycle.WorkspaceAssociation(ctx, primary, entry.ID)
	if err != nil || !exists {
		return MutationResponse{}, false, err
	}
	response, err := coordinator.Privileged.WorkspacePublish(ctx, HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
	if err != nil {
		return MutationResponse{}, false, err
	}
	if !response.OK || response.WorkspaceUsername != workspaceUsername {
		return MutationResponse{}, false, errors.New("workspace helper returned an inconsistent existing workspace")
	}
	return response, true, nil
}

func (coordinator Coordinator) setupNewWorkspace(
	ctx context.Context,
	primary Account,
	entry CatalogEntry,
	request SetupRequest,
) (response MutationResponse, returnErr error) {
	if _, err := coordinator.Platform.ReadAuthorizedKeys(primary); err != nil {
		return MutationResponse{}, fmt.Errorf("setup requires a public key in ~/.ssh/authorized_keys: %w", err)
	}
	staging := coordinator.Platform.StagingPath(primary, request.ID)
	if err := coordinator.Platform.ResetStaging(primary, request.ID); err != nil {
		return MutationResponse{}, err
	}
	defer func() {
		if cleanupErr := coordinator.Platform.CleanupStaging(primary, request.ID); cleanupErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("clean clone staging directory: %w", cleanupErr))
		}
	}()
	if err := coordinator.cloneForSetup(ctx, entry, staging, request); err != nil {
		return MutationResponse{}, err
	}
	if err := coordinator.Platform.PrepareStaging(primary, request.ID); err != nil {
		return MutationResponse{}, err
	}
	response, err := coordinator.Privileged.WorkspacePublish(ctx, HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
	if err != nil {
		return MutationResponse{}, err
	}
	if !response.OK || response.WorkspaceUsername == "" {
		return MutationResponse{}, errors.New("workspace helper returned an incomplete result")
	}
	return response, nil
}

func (coordinator Coordinator) cloneForSetup(ctx context.Context, entry CatalogEntry, staging string, request SetupRequest) error {
	return coordinator.Cloner.Clone(ctx, entry.CanonicalURL, staging)
}

func (coordinator Coordinator) projectResult(ctx context.Context, primary Account, entry CatalogEntry) (MutationResponse, error) {
	view, err := coordinator.projectView(ctx, primary, entry)
	if err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true, Project: &view}, nil
}

func (coordinator Coordinator) projectView(ctx context.Context, primary Account, entry CatalogEntry) (ProjectView, error) {
	username, _, err := coordinator.Lifecycle.WorkspaceAssociation(ctx, primary, entry.ID)
	if err != nil {
		return ProjectView{}, err
	}
	return ProjectView{CatalogEntry: entry, WorkspaceUsername: username}, nil
}
