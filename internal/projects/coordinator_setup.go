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
	setupLock, err := coordinator.Platform.SetupLock(primary, request.ID)
	if err != nil {
		return MutationResponse{}, fmt.Errorf("lock project setup: %w", err)
	}
	response, setupErr := coordinator.setupWithOperationLock(ctx, request)
	return response, closeLockWithError(setupLock, setupErr, "project setup")
}

func (coordinator Coordinator) setupWithOperationLock(ctx context.Context, request SetupRequest) (MutationResponse, error) {
	lock, err := coordinator.Platform.WorkspaceOperationSharedLock()
	if err != nil {
		return MutationResponse{}, fmt.Errorf("lock workspace operations: %w", err)
	}
	response, setupErr := coordinator.setupLocked(ctx, request)
	return response, closeLockWithError(lock, setupErr, "workspace operations")
}

func (coordinator Coordinator) setupLocked(ctx context.Context, request SetupRequest) (MutationResponse, error) {
	entry, err := coordinator.Catalog.Get(request.ID)
	if err != nil {
		return MutationResponse{}, err
	}
	prepared, err := coordinator.prepareWorkspace(ctx, entry)
	if err != nil {
		return MutationResponse{}, err
	}
	response, err := coordinator.publishWorkspace(ctx, entry)
	if err != nil {
		return MutationResponse{}, fmt.Errorf(
			"workspace %s and its outbound Git key were retained; its public key is %q. If repository authorization caused the clone failure, register that key with the authoritative Git host, then retry setup: %w",
			prepared.WorkspaceUsername,
			prepared.WorkspacePublicKey,
			err,
		)
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

func (coordinator Coordinator) publishWorkspace(ctx context.Context, entry CatalogEntry) (MutationResponse, error) {
	return coordinator.Privileged.WorkspacePublish(ctx, HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
}
