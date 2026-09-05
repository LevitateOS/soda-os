package projects

import (
	"context"
	"fmt"
	"io"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/LevitateOS/soda-os/internal/strictjson"
)

func (coordinator Coordinator) executeInspect(ctx context.Context, input io.Reader) (WorkspaceInspectionResponse, error) {
	var request ProjectRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return WorkspaceInspectionResponse{}, err
	}
	if err := catalog.ValidateID(request.ID); err != nil {
		return WorkspaceInspectionResponse{}, err
	}
	return coordinator.privileged.WorkspaceInspect(ctx, request)
}

// workspaceInspect reads only the caller's derived account. Keep private-home
// inspection out of catalog listing and use the same deletion/authorization
// boundary as setup, without preparing a workspace as a side effect.
func (helper Helper) workspaceInspect(ctx context.Context, actor linuxhost.PKExecIdentity, input io.Reader) (WorkspaceInspectionResponse, error) {
	var request ProjectRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return WorkspaceInspectionResponse{}, err
	}
	lock, err := helper.operationLocks.Shared()
	if err != nil {
		return WorkspaceInspectionResponse{}, fmt.Errorf("lock workspace operations: %w", err)
	}
	primary, _, err := helper.authorizeActor(ctx, actor)
	if err != nil {
		return WorkspaceInspectionResponse{}, closeLockWithError(lock, err, "workspace operations")
	}
	entry, err := helper.store.Get(request.ID)
	if err != nil {
		return WorkspaceInspectionResponse{}, closeLockWithError(lock, err, "workspace operations")
	}
	inspection, err := helper.workspaces.Inspect(ctx, helper.repository, primary, entry)
	if err = closeLockWithError(lock, err, "workspace operations"); err != nil {
		return WorkspaceInspectionResponse{}, err
	}
	return WorkspaceInspectionResponse{OK: true, Workspace: inspection}, nil
}
