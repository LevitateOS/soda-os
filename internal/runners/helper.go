package runners

import (
	"context"
	"fmt"
	"io"
)

type Lifecycle interface {
	Create(context.Context, CreateRequest) error
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Restart(context.Context, string) error
	Remove(context.Context, string) error
}

type Helper struct {
	Authorizer Authorizer
	Lifecycle  Lifecycle
}

func (helper Helper) Execute(ctx context.Context, username, action string, input io.Reader) (MutationResponse, error) {
	if err := helper.Authorizer.RequireAdministrator(ctx, username); err != nil {
		return MutationResponse{}, err
	}
	if action == "create" {
		return helper.create(ctx, input)
	}
	return helper.mutate(ctx, action, input)
}

func (helper Helper) create(ctx context.Context, input io.Reader) (MutationResponse, error) {
	var request CreateRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if err := request.Validate(); err != nil {
		return MutationResponse{}, err
	}
	if err := helper.Lifecycle.Create(ctx, request); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}

func (helper Helper) mutate(ctx context.Context, action string, input io.Reader) (MutationResponse, error) {
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
		err = helper.Lifecycle.Start(ctx, request.ID)
	case "stop":
		err = helper.Lifecycle.Stop(ctx, request.ID)
	case "restart":
		err = helper.Lifecycle.Restart(ctx, request.ID)
	case "remove":
		err = helper.Lifecycle.Remove(ctx, request.ID)
	default:
		return MutationResponse{}, fmt.Errorf("unsupported runner helper action %q", action)
	}
	if err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}
