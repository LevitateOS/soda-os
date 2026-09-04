package projects

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
)

type Helper struct {
	Lifecycle Lifecycle
}

func (helper Helper) Execute(ctx context.Context, actor linuxhost.PKExecIdentity, action string, input io.Reader) (MutationResponse, error) {
	account, _, err := helper.Lifecycle.AuthorizePrimary(ctx, actor.Username)
	if err != nil {
		return MutationResponse{}, err
	}
	if account.UID != actor.UID {
		return MutationResponse{}, errors.New("PKEXEC_UID no longer matches the authorized Linux account")
	}
	return helper.dispatch(ctx, account, action, input)
}

func (helper Helper) dispatch(ctx context.Context, account linuxhost.Account, action string, input io.Reader) (MutationResponse, error) {
	handlers := map[string]func() (MutationResponse, error){
		"catalog-add":       func() (MutationResponse, error) { return helper.catalogAdd(input) },
		"catalog-edit":      func() (MutationResponse, error) { return helper.catalogEdit(input) },
		"workspace-publish": func() (MutationResponse, error) { return helper.workspacePublish(ctx, account.Username, input) },
		"workspace-prepare": func() (MutationResponse, error) { return helper.workspacePrepare(ctx, account.Username, input) },
		"workspace-remove":  func() (MutationResponse, error) { return helper.workspaceRemove(ctx, account.Username, input) },
		"project-remove":    func() (MutationResponse, error) { return helper.projectRemove(ctx, account.Username, input) },
		"human-delete":      func() (MutationResponse, error) { return helper.humanDelete(ctx, account.Username, input) },
	}
	handler, found := handlers[action]
	if !found {
		return MutationResponse{}, fmt.Errorf("unsupported workspace helper action %q", action)
	}
	return handler()
}

func (helper Helper) workspaceRemove(ctx context.Context, actorUsername string, input io.Reader) (MutationResponse, error) {
	var request ProjectRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if err := helper.Lifecycle.RemoveWorkspace(ctx, actorUsername, request.ID); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}

func (helper Helper) catalogAdd(input io.Reader) (MutationResponse, error) {
	var request HelperCatalogRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if err := helper.Lifecycle.Catalog.Add(request.CatalogEntry); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}

func (helper Helper) catalogEdit(input io.Reader) (MutationResponse, error) {
	var request HelperEditRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if err := helper.Lifecycle.Catalog.Edit(request); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}

func (helper Helper) workspacePublish(ctx context.Context, actorUsername string, input io.Reader) (MutationResponse, error) {
	var request HelperWorkspaceRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	username, err := helper.Lifecycle.CompleteWorkspace(ctx, actorUsername, request)
	if err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true, WorkspaceUsername: username}, nil
}

func (helper Helper) workspacePrepare(ctx context.Context, actorUsername string, input io.Reader) (MutationResponse, error) {
	var request HelperWorkspaceRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	preparation, err := helper.Lifecycle.PrepareWorkspace(ctx, actorUsername, request)
	if err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true, WorkspaceUsername: preparation.Username, WorkspacePublicKey: preparation.PublicKey}, nil
}

func (helper Helper) projectRemove(ctx context.Context, actorUsername string, input io.Reader) (MutationResponse, error) {
	var request ProjectRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if err := helper.Lifecycle.RemoveProject(ctx, actorUsername, request.ID); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}

func (helper Helper) humanDelete(ctx context.Context, actorUsername string, input io.Reader) (MutationResponse, error) {
	var request HelperHumanRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if err := helper.Lifecycle.DeleteHuman(ctx, actorUsername, request.Username); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}
