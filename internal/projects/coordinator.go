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
	WorkspacePrepare(context.Context, HelperWorkspaceRequest) (MutationResponse, error)
	WorkspacePublish(context.Context, HelperWorkspaceRequest) (MutationResponse, error)
	WorkspaceRemove(context.Context, ProjectRequest) error
	ToolsInstall(context.Context, HelperToolRequest) error
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

func (invoker PKExecInvoker) WorkspacePrepare(ctx context.Context, request HelperWorkspaceRequest) (MutationResponse, error) {
	return invoker.mutation(ctx, "workspace-prepare", request)
}

func (invoker PKExecInvoker) WorkspaceRemove(ctx context.Context, request ProjectRequest) error {
	_, err := invoker.mutation(ctx, "workspace-remove", request)
	return err
}

func (invoker PKExecInvoker) ToolsInstall(ctx context.Context, request HelperToolRequest) error {
	_, err := invoker.mutation(ctx, "tools-install", request)
	return err
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
	Endpoints  EndpointSource
}

func (coordinator Coordinator) Execute(ctx context.Context, actorUsername, action string, input io.Reader) (any, error) {
	primary, uidMin, err := coordinator.Lifecycle.AuthorizePrimary(ctx, actorUsername)
	if err != nil {
		return nil, err
	}
	return coordinator.dispatch(ctx, primary, uidMin, action, input)
}

func (coordinator Coordinator) dispatch(ctx context.Context, primary Account, uidMin int, action string, input io.Reader) (any, error) {
	handlers := map[string]func() (any, error){
		"list":             func() (any, error) { return coordinator.executeList(ctx, primary, uidMin, input) },
		"add-existing":     func() (any, error) { return coordinator.executeAddExisting(ctx, primary, input) },
		"create-forgejo":   func() (any, error) { return coordinator.executeCreateForgejo(ctx, primary, input) },
		"edit":             func() (any, error) { return coordinator.executeEdit(ctx, primary, input) },
		"setup":            func() (any, error) { return coordinator.executeSetup(ctx, primary, input) },
		"remove-workspace": func() (any, error) { return coordinator.executeRemoveWorkspace(ctx, input) },
		"install-tools":    func() (any, error) { return coordinator.executeInstallTools(ctx, input) },
		"remove":           func() (any, error) { return coordinator.executeRemove(ctx, input) },
		"delete-human":     func() (any, error) { return coordinator.executeDeleteHuman(ctx, input) },
		"add-person":       func() (any, error) { return coordinator.executeAddPerson(ctx, primary, uidMin, input) },
	}
	handler, found := handlers[action]
	if !found {
		return nil, fmt.Errorf("unsupported soda-projects action %q", action)
	}
	return handler()
}

func (coordinator Coordinator) executeInstallTools(ctx context.Context, input io.Reader) (any, error) {
	var request ToolRequest
	if err := DecodeRequest(input, &request); err != nil {
		return nil, err
	}
	if request.Scope != "workspace" && request.Scope != "project" {
		return nil, errors.New("mise tool scope must be workspace or project")
	}
	if len(request.Tools) == 0 {
		return nil, errors.New("at least one mise tool is required")
	}
	if err := ValidateToolSelections(request.Tools); err != nil {
		return nil, err
	}
	if err := coordinator.Privileged.ToolsInstall(ctx, HelperToolRequest(request)); err != nil {
		return nil, err
	}
	return MutationResponse{OK: true}, nil
}

func (coordinator Coordinator) executeRemoveWorkspace(ctx context.Context, input io.Reader) (any, error) {
	var request ProjectRequest
	if err := DecodeRequest(input, &request); err != nil {
		return nil, err
	}
	if err := coordinator.Privileged.WorkspaceRemove(ctx, request); err != nil {
		return nil, err
	}
	return MutationResponse{OK: true}, nil
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
	entry := request.CatalogEntry
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	if err := coordinator.Privileged.CatalogAdd(ctx, request); err != nil {
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
	entry := request.CatalogEntry
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	if err := coordinator.Privileged.CatalogEdit(ctx, request); err != nil {
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
	if err = coordinator.Privileged.CatalogAdd(ctx, HelperCatalogRequest{CatalogEntry: entry}); err != nil {
		return MutationResponse{}, fmt.Errorf("repository was created at %s but catalog publication failed: %w", created.CanonicalURL, err)
	}
	return coordinator.projectResult(ctx, primary, entry)
}

func (coordinator Coordinator) projectResult(ctx context.Context, primary Account, entry CatalogEntry) (MutationResponse, error) {
	view, err := coordinator.projectView(ctx, primary, entry)
	if err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true, Project: &view}, nil
}

func (coordinator Coordinator) projectView(ctx context.Context, primary Account, entry CatalogEntry) (ProjectView, error) {
	username, ready, err := coordinator.Lifecycle.WorkspaceAssociation(ctx, primary, entry.ID)
	if err != nil {
		return ProjectView{}, err
	}
	return ProjectView{CatalogEntry: entry, WorkspaceUsername: username, WorkspaceReady: ready}, nil
}
