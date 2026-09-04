package runners

import (
	"context"
	"fmt"
	"io"
)

type LocalReader interface {
	List(context.Context) ([]RunnerView, error)
}

type PrivilegedRunners interface {
	Create(context.Context, CreateRequest) error
	Start(context.Context, RunnerRequest) error
	Stop(context.Context, RunnerRequest) error
	Restart(context.Context, RunnerRequest) error
	Remove(context.Context, RunnerRequest) error
}

type ForgejoEndpoint interface {
	Endpoints(context.Context) (forgejoURL, sshHost string, err error)
}

type Coordinator struct {
	Authorizer Authorizer
	Local      LocalReader
	Privileged PrivilegedRunners
	Endpoints  ForgejoEndpoint
}

func (coordinator Coordinator) Execute(ctx context.Context, username, action string, input io.Reader) (any, error) {
	if err := coordinator.Authorizer.RequireAdministrator(ctx, username); err != nil {
		return nil, err
	}
	switch action {
	case "list":
		return coordinator.list(ctx, input)
	case "create":
		return coordinator.create(ctx, input)
	case "start", "stop", "restart", "remove":
		return coordinator.mutate(ctx, action, input)
	default:
		return nil, fmt.Errorf("unsupported soda-runners action %q", action)
	}
}

func (coordinator Coordinator) list(ctx context.Context, input io.Reader) (ListResponse, error) {
	var request EmptyRequest
	if err := DecodeRequest(input, &request); err != nil {
		return ListResponse{}, err
	}
	views, err := coordinator.Local.List(ctx)
	if err != nil {
		return ListResponse{}, err
	}
	forgejoURL, _, err := coordinator.Endpoints.Endpoints(ctx)
	if err != nil {
		return ListResponse{}, fmt.Errorf("resolve bundled Forgejo URL: %w", err)
	}
	response := ListResponse{Runners: views, RunnerCount: len(views), TotalCapacity: len(views) * RunnerCapacity, ForgejoURL: forgejoURL}
	for _, view := range views {
		if view.Service.Active == "active" && view.Service.Sub == "running" {
			response.ActiveListeners++
		}
	}
	return response, nil
}

func (coordinator Coordinator) create(ctx context.Context, input io.Reader) (MutationResponse, error) {
	var request CreateRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if request.Provider == ProviderForgejo {
		forgejoURL, _, err := coordinator.Endpoints.Endpoints(ctx)
		if err != nil {
			return MutationResponse{}, fmt.Errorf("resolve bundled Forgejo URL: %w", err)
		}
		request.RegistrationURL = forgejoURL
	}
	if err := request.Validate(); err != nil {
		return MutationResponse{}, err
	}
	if err := coordinator.Privileged.Create(ctx, request); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}

func (coordinator Coordinator) mutate(ctx context.Context, action string, input io.Reader) (MutationResponse, error) {
	var request RunnerRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if err := ValidateID(request.ID); err != nil {
		return MutationResponse{}, err
	}
	var err error
	switch action {
	case "start":
		err = coordinator.Privileged.Start(ctx, request)
	case "stop":
		err = coordinator.Privileged.Stop(ctx, request)
	case "restart":
		err = coordinator.Privileged.Restart(ctx, request)
	case "remove":
		err = coordinator.Privileged.Remove(ctx, request)
	}
	if err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}
