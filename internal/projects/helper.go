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
	switch action {
	case "catalog-add":
		return helper.catalogAdd(input)
	case "catalog-edit":
		return helper.catalogEdit(input)
	case "workspace-publish":
		return helper.workspacePublish(ctx, actor.Username, input)
	case "workspace-prepare":
		return helper.workspacePrepare(ctx, actor.Username, input)
	case "workspace-remove":
		return helper.workspaceRemove(ctx, actor.Username, input)
	case "tools-install":
		return helper.toolsInstall(ctx, actor.Username, input)
	case "project-remove":
		return helper.projectRemove(ctx, actor.Username, input)
	case "human-delete":
		return helper.humanDelete(ctx, actor.Username, input)
	case "human-create":
		return helper.humanCreate(ctx, account, input)
	case "human-publish":
		return helper.humanPublish(ctx, account, input)
	default:
		return MutationResponse{}, fmt.Errorf("unsupported workspace helper action %q", action)
	}
}

func (helper Helper) toolsInstall(ctx context.Context, actorUsername string, input io.Reader) (MutationResponse, error) {
	var request HelperToolRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if err := helper.Lifecycle.InstallTools(ctx, actorUsername, request); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
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

func (helper Helper) humanCreate(ctx context.Context, actor Account, input io.Reader) (MutationResponse, error) {
	uidMin, err := helper.Lifecycle.Platform.UIDMin()
	if err != nil {
		return MutationResponse{}, err
	}
	if !actor.IsAdministrator(uidMin) {
		return MutationResponse{}, errors.New("administrator status is required")
	}
	var request HelperHumanCreateRequest
	if err = DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	if _, err = helper.Lifecycle.Platform.CreatePrimary(ctx, request.Username, request.Password); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}

func (helper Helper) humanPublish(ctx context.Context, actor Account, input io.Reader) (MutationResponse, error) {
	uidMin, err := helper.Lifecycle.Platform.UIDMin()
	if err != nil {
		return MutationResponse{}, err
	}
	if !actor.IsAdministrator(uidMin) {
		return MutationResponse{}, errors.New("administrator status is required")
	}
	var request HelperHumanPublishRequest
	if err = DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	key, err := canonicalAuthorizedKey(request.AuthorizedKey)
	if err != nil {
		return MutationResponse{}, err
	}
	if err = helper.Lifecycle.Platform.PublishHuman(ctx, request.Username, []byte(key)); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}

func (helper Helper) catalogAdd(input io.Reader) (MutationResponse, error) {
	var request HelperCatalogRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	entry := CatalogEntry{ID: request.ID, DisplayName: request.DisplayName, CanonicalURL: request.CanonicalURL}
	if err := helper.Lifecycle.Catalog.Add(entry); err != nil {
		return MutationResponse{}, err
	}
	return MutationResponse{OK: true}, nil
}

func (helper Helper) catalogEdit(input io.Reader) (MutationResponse, error) {
	var request HelperCatalogRequest
	if err := DecodeRequest(input, &request); err != nil {
		return MutationResponse{}, err
	}
	entry := CatalogEntry{ID: request.ID, DisplayName: request.DisplayName, CanonicalURL: request.CanonicalURL}
	if err := helper.Lifecycle.Catalog.Edit(entry); err != nil {
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
