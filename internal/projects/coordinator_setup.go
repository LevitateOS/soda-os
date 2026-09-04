package projects

import (
	"context"
	"errors"
	"fmt"
)

func (coordinator Coordinator) setup(ctx context.Context, primary Account, request SetupRequest) (MutationResponse, error) {
	if !projectIDPattern.MatchString(request.ID) {
		return MutationResponse{}, errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	if err := ValidateToolSelections(request.WorkspaceTools); err != nil {
		return MutationResponse{}, err
	}
	if err := ValidateToolSelections(request.ProjectTools); err != nil {
		return MutationResponse{}, err
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
	forgejoURL, sshHost, err := coordinator.Endpoints.Endpoints(ctx)
	if err != nil {
		return MutationResponse{}, err
	}
	bundledForgejo, err := remoteUsesSSHHost(entry.CanonicalURL, sshHost)
	if err != nil {
		return MutationResponse{}, fmt.Errorf("identify repository SSH host: %w", err)
	}
	if bundledForgejo {
		return coordinator.setupBundledForgejoWorkspace(ctx, primary, entry, request, forgejoURL)
	}
	return coordinator.setupExternalWorkspace(ctx, entry, request)
}

func (coordinator Coordinator) setupBundledForgejoWorkspace(ctx context.Context, primary Account, entry CatalogEntry, request SetupRequest, forgejoURL string) (MutationResponse, error) {
	if err := validateHumanPassword(request.ForgejoPassword); err != nil {
		return MutationResponse{}, err
	}
	prepared, err := coordinator.prepareWorkspace(ctx, entry)
	if err != nil {
		return MutationResponse{}, err
	}
	if err = coordinator.registerWorkspaceKey(ctx, primary, request, prepared, forgejoURL); err != nil {
		return MutationResponse{}, err
	}
	response, err := coordinator.publishWorkspace(ctx, entry, request)
	if err != nil {
		return MutationResponse{}, err
	}
	return completeWorkspaceSetup(response, prepared)
}

func (coordinator Coordinator) setupExternalWorkspace(ctx context.Context, entry CatalogEntry, request SetupRequest) (MutationResponse, error) {
	prepared, err := coordinator.prepareWorkspace(ctx, entry)
	if err != nil {
		return MutationResponse{}, err
	}
	response, err := coordinator.publishExternalWorkspace(ctx, entry, request, prepared)
	if err != nil {
		return MutationResponse{}, err
	}
	return completeWorkspaceSetup(response, prepared)
}

func (coordinator Coordinator) prepareWorkspace(ctx context.Context, entry CatalogEntry) (MutationResponse, error) {
	prepared, err := coordinator.Privileged.WorkspacePrepare(ctx, HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
	if err != nil {
		return MutationResponse{}, err
	}
	if err = validateWorkspacePreparation(prepared); err != nil {
		return MutationResponse{}, err
	}
	return prepared, nil
}

func validateWorkspacePreparation(prepared MutationResponse) error {
	if !prepared.OK || prepared.WorkspaceUsername == "" || prepared.WorkspacePublicKey == "" {
		return errors.New("workspace helper returned incomplete outbound Git key evidence")
	}
	return nil
}

func completeWorkspaceSetup(response, prepared MutationResponse) (MutationResponse, error) {
	if !response.OK || response.WorkspaceUsername != prepared.WorkspaceUsername {
		return MutationResponse{}, errors.New("workspace helper returned an incomplete clone result")
	}
	return response, nil
}

func (coordinator Coordinator) publishWorkspace(ctx context.Context, entry CatalogEntry, request SetupRequest) (MutationResponse, error) {
	return coordinator.Privileged.WorkspacePublish(ctx, HelperWorkspaceRequest{
		ID: entry.ID, CanonicalURL: entry.CanonicalURL,
		WorkspaceTools: request.WorkspaceTools, ProjectTools: request.ProjectTools,
	})
}

func (coordinator Coordinator) publishExternalWorkspace(ctx context.Context, entry CatalogEntry, request SetupRequest, prepared MutationResponse) (MutationResponse, error) {
	response, err := coordinator.publishWorkspace(ctx, entry, request)
	if err == nil {
		return response, nil
	}
	return MutationResponse{}, fmt.Errorf(
		"workspace %s and its outbound Git key were retained; the external Git host owns access, so register public key %q there and retry setup: %w",
		prepared.WorkspaceUsername,
		prepared.WorkspacePublicKey,
		err,
	)
}

func (coordinator Coordinator) registerWorkspaceKey(ctx context.Context, primary Account, request SetupRequest, prepared MutationResponse, forgejoURL string) error {
	err := coordinator.Forgejo.RegisterPublicKey(ctx, ForgejoKeyRequest{
		BaseURL: forgejoURL, Username: primary.Username, Password: request.ForgejoPassword,
		PublicKey: prepared.WorkspacePublicKey, Title: "Soda OS workspace " + prepared.WorkspaceUsername,
	})
	if err != nil {
		return fmt.Errorf("workspace %s and its local outbound Git key were retained; Forgejo key registration can be retried: %w", prepared.WorkspaceUsername, err)
	}
	return nil
}
