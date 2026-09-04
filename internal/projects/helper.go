package projects

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
)

type Helper struct {
	Lifecycle Lifecycle
}

type PKExecIdentity struct {
	Username string
	UID      int
}

func (helper Helper) Execute(ctx context.Context, actor PKExecIdentity, action string, input io.Reader) (MutationResponse, error) {
	account, _, err := helper.Lifecycle.AuthorizePrimary(ctx, actor.Username)
	if err != nil {
		return MutationResponse{}, err
	}
	if account.UID != actor.UID {
		return MutationResponse{}, errors.New("PKEXEC_UID no longer matches the authorized Linux account")
	}
	return helper.dispatch(ctx, account, action, input)
}

func (helper Helper) dispatch(ctx context.Context, account Account, action string, input io.Reader) (MutationResponse, error) {
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
	var request HelperCatalogRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if err := helper.Lifecycle.Catalog.Edit(request.CatalogEntry); err != nil {
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

func PKExecCaller() (PKExecIdentity, error) {
	if os.Geteuid() != 0 {
		return PKExecIdentity{}, errors.New("workspace helper must run with effective UID 0")
	}
	rawUID, present := os.LookupEnv("PKEXEC_UID")
	if !present || rawUID == "" {
		return PKExecIdentity{}, errors.New("PKEXEC_UID is required")
	}
	uid, err := strconv.ParseUint(rawUID, 10, 32)
	if err != nil || uid == 0 {
		return PKExecIdentity{}, errors.New("PKEXEC_UID must identify a non-root caller")
	}
	account, err := user.LookupId(strconv.FormatUint(uid, 10))
	if err != nil {
		return PKExecIdentity{}, errors.New("resolve PKEXEC_UID")
	}
	return PKExecIdentity{Username: account.Username, UID: int(uid)}, nil
}
