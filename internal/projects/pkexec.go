package projects

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LevitateOS/soda-os/internal/projects/catalog"
)

const (
	systemPKExecBinary = "/usr/bin/pkexec"
	systemHelperPath   = "/usr/libexec/soda/soda-workspace-helper"
)

// PKExecInvoker invokes the one fixed privileged Projects helper. Each method
// has the wire result for its single action; there is no generic privileged
// lifecycle contract.
type PKExecInvoker struct {
	binary string
	helper string
}

func NewSystemPKExecInvoker() PKExecInvoker {
	return PKExecInvoker{binary: systemPKExecBinary, helper: systemHelperPath}
}

func NewPKExecInvoker(binary, helper string) (PKExecInvoker, error) {
	if !filepath.IsAbs(binary) {
		return PKExecInvoker{}, errors.New("pkexec binary path must be absolute")
	}
	if !filepath.IsAbs(helper) {
		return PKExecInvoker{}, errors.New("workspace helper path must be absolute")
	}
	return PKExecInvoker{binary: binary, helper: helper}, nil
}

func (invoker PKExecInvoker) CatalogAdd(ctx context.Context, request catalog.Entry) (ProjectMutationResponse, error) {
	var mutation ProjectMutationResponse
	if err := invoker.invoke(ctx, "catalog-add", request, &mutation); err != nil {
		return ProjectMutationResponse{}, err
	}
	if !mutation.OK {
		return ProjectMutationResponse{}, privilegedIncomplete("catalog-add")
	}
	return mutation, nil
}

func (invoker PKExecInvoker) CatalogEdit(ctx context.Context, request catalog.Edit) (ProjectMutationResponse, error) {
	var response ProjectMutationResponse
	if err := invoker.invoke(ctx, "catalog-edit", request, &response); err != nil {
		return ProjectMutationResponse{}, err
	}
	if !response.OK {
		return ProjectMutationResponse{}, privilegedIncomplete("catalog-edit")
	}
	return response, nil
}

func (invoker PKExecInvoker) WorkspacePrepare(ctx context.Context, request HelperWorkspaceRequest) (WorkspacePreparationResponse, error) {
	var response WorkspacePreparationResponse
	if err := invoker.invoke(ctx, "workspace-prepare", request, &response); err != nil {
		return WorkspacePreparationResponse{}, err
	}
	if !response.OK {
		return WorkspacePreparationResponse{}, privilegedIncomplete("workspace-prepare")
	}
	return response, nil
}

func (invoker PKExecInvoker) WorkspacePublish(ctx context.Context, request HelperWorkspaceRequest) (WorkspacePublicationResponse, error) {
	var response WorkspacePublicationResponse
	if err := invoker.invoke(ctx, "workspace-publish", request, &response); err != nil {
		return WorkspacePublicationResponse{}, err
	}
	if !response.OK {
		return WorkspacePublicationResponse{}, privilegedIncomplete("workspace-publish")
	}
	return response, nil
}

func (invoker PKExecInvoker) WorkspaceRemove(ctx context.Context, request ProjectRequest) error {
	return invoker.success(ctx, "workspace-remove", request)
}

func (invoker PKExecInvoker) ProjectRemove(ctx context.Context, request ProjectRequest) error {
	return invoker.success(ctx, "project-remove", request)
}

func (invoker PKExecInvoker) HumanDelete(ctx context.Context, request HelperHumanRequest) error {
	return invoker.success(ctx, "human-delete", request)
}

func (invoker PKExecInvoker) success(ctx context.Context, action string, request any) error {
	var response SuccessResponse
	if err := invoker.invoke(ctx, action, request, &response); err != nil {
		return err
	}
	if !response.OK {
		return privilegedIncomplete(action)
	}
	return nil
}

func privilegedIncomplete(action string) error {
	return fmt.Errorf("privileged %s did not complete", action)
}

func (invoker PKExecInvoker) invoke(ctx context.Context, action string, request, response any) error {
	if invoker.binary == "" || invoker.helper == "" {
		return errors.New("pkexec invoker was not constructed")
	}
	contents, err := json.Marshal(request)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, invoker.binary, "--disable-internal-agent", invoker.helper, action)
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
	decoder := json.NewDecoder(&stdout)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(response); err != nil {
		return fmt.Errorf("decode privileged %s result: %w", action, err)
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode privileged %s result: response must contain exactly one JSON value", action)
	}
	return nil
}
