package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
)

func (coordinator Coordinator) setup(ctx context.Context, primary linuxhost.Account, request SetupRequest) (SetupResponse, error) {
	if err := catalog.ValidateID(request.ID); err != nil {
		return SetupResponse{}, err
	}
	entry, err := coordinator.store.Get(request.ID)
	if err != nil {
		return SetupResponse{}, err
	}
	setupLock, err := coordinator.setupLocks.Lock(primary, entry)
	if err != nil {
		return SetupResponse{}, fmt.Errorf("lock project setup: %w", err)
	}
	response, setupErr := coordinator.setupWithOperationLock(ctx, request)
	return response, closeLockWithError(setupLock, setupErr, "project setup")
}

func (coordinator Coordinator) setupWithOperationLock(ctx context.Context, request SetupRequest) (SetupResponse, error) {
	lock, err := coordinator.operationLocks.Shared()
	if err != nil {
		return SetupResponse{}, fmt.Errorf("lock workspace operations: %w", err)
	}
	response, setupErr := coordinator.setupLocked(ctx, request)
	return response, closeLockWithError(lock, setupErr, "workspace operations")
}

func (coordinator Coordinator) setupLocked(ctx context.Context, request SetupRequest) (SetupResponse, error) {
	entry, err := coordinator.store.Get(request.ID)
	if err != nil {
		return SetupResponse{}, err
	}
	prepared, err := coordinator.prepareWorkspace(ctx, entry)
	if err != nil {
		return SetupResponse{}, err
	}
	published, err := coordinator.publishWorkspace(ctx, entry)
	if err != nil {
		return SetupResponse{}, fmt.Errorf(
			"workspace %s and its outbound Git key were retained; its public key is %q. If repository authorization caused the clone failure, register that key with the authoritative Git host, then retry setup: %w",
			prepared.WorkspaceUsername,
			prepared.WorkspacePublicKey,
			err,
		)
	}
	return completeWorkspaceSetup(published, prepared)
}

func (coordinator Coordinator) prepareWorkspace(ctx context.Context, entry catalog.Entry) (WorkspacePreparationResponse, error) {
	prepared, err := coordinator.privileged.WorkspacePrepare(ctx, HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
	if err != nil {
		return WorkspacePreparationResponse{}, err
	}
	if !prepared.OK || prepared.WorkspaceUsername == "" || prepared.WorkspacePublicKey == "" {
		return WorkspacePreparationResponse{}, errors.New("workspace helper returned incomplete outbound Git key evidence")
	}
	return prepared, nil
}

func (coordinator Coordinator) publishWorkspace(ctx context.Context, entry catalog.Entry) (WorkspacePublicationResponse, error) {
	return coordinator.privileged.WorkspacePublish(ctx, HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
}

func completeWorkspaceSetup(published WorkspacePublicationResponse, prepared WorkspacePreparationResponse) (SetupResponse, error) {
	if !published.OK || published.WorkspaceUsername != prepared.WorkspaceUsername {
		return SetupResponse{}, errors.New("workspace helper returned an incomplete clone result")
	}
	return SetupResponse{OK: true, WorkspaceUsername: published.WorkspaceUsername}, nil
}
