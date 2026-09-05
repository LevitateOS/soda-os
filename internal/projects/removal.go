package projects

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/LevitateOS/soda-os/internal/projects/people"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
	"github.com/LevitateOS/soda-os/internal/strictjson"
)

type RemovalInspectionRequest struct {
	Action string `json:"action"`
	Target string `json:"target"`
}

type RemoveProjectRequest struct {
	ID       string `json:"id"`
	Expected string `json:"expected"`
}

type RemovalAccount struct {
	UID             int    `json:"uid"`
	Username        string `json:"username"`
	PrimaryUsername string `json:"primary_username"`
	ProjectID       string `json:"project_id"`
	Home            string `json:"home"`
}

type RemovalPreview struct {
	Action         string           `json:"action"`
	Target         string           `json:"target"`
	Revision       string           `json:"revision"`
	Accounts       []RemovalAccount `json:"accounts"`
	CatalogPresent bool             `json:"catalog_present"`
}

type RemovalInspectionResponse struct {
	OK      bool           `json:"ok"`
	Preview RemovalPreview `json:"preview"`
}

// A failed operation can still deliver a complete receipt. Neither failed
// native deletion nor catalog publication implies that its data remains intact.
type RemovalResponse struct {
	OK      bool                     `json:"ok"`
	Result  linuxhost.DeletionResult `json:"result"`
	Catalog string                   `json:"catalog"`
	Problem string                   `json:"problem"`
}

type removalScope struct {
	Action   string
	Target   string
	Accounts []linuxhost.Account
	Project  *catalog.Entry
}

func (scope removalScope) preview() RemovalPreview {
	// Confirm native identities and repository identity, not cosmetic catalog metadata.
	identity := struct {
		Action, Target, Repository string
		Accounts                   []linuxhost.Account
	}{Action: scope.Action, Target: scope.Target, Accounts: scope.Accounts}
	if scope.Project != nil {
		identity.Repository = scope.Project.CanonicalURL
	}
	contents, _ := json.Marshal(identity) // Only strings, integers, and native boolean groups.
	preview := RemovalPreview{Action: scope.Action, Target: scope.Target, Revision: fmt.Sprintf("%x", sha256.Sum256(contents)), Accounts: []RemovalAccount{}, CatalogPresent: scope.Project != nil}
	for _, account := range scope.Accounts {
		primary, projectID, _ := workspace.ParseMarker(account.GECOS)
		if projectID == "" {
			primary = account.Username
		}
		preview.Accounts = append(preview.Accounts, RemovalAccount{UID: account.UID, Username: account.Username, PrimaryUsername: primary, ProjectID: projectID, Home: account.Home})
	}
	return preview
}

func (helper Helper) removalScope(ctx context.Context, identity linuxhost.PKExecIdentity, request RemovalInspectionRequest) (removalScope, error) {
	scope := removalScope{Action: request.Action, Target: request.Target}
	actor, uidMin, err := helper.authorizeActor(ctx, identity)
	if err != nil {
		return scope, err
	}
	switch request.Action {
	case "remove-workspace":
		scope.Accounts, err = helper.remover.Targets(ctx, actor, request.Target)
	case "remove":
		if !people.IsAdministrator(actor, uidMin) {
			return scope, errors.New("administrator status is required")
		}
		scope.Project, err = helper.removalProject(request.Target)
		if err == nil {
			scope.Accounts, err = helper.remover.ProjectTargets(ctx, request.Target, uidMin)
		}
	case "delete-human":
		scope.Accounts, err = helper.people.Targets(ctx, actor, uidMin, request.Target)
	default:
		err = errors.New("unsupported removal action")
	}
	return scope, err
}

func (helper Helper) removalProject(id string) (*catalog.Entry, error) {
	if err := catalog.ValidateID(id); err != nil {
		return nil, err
	}
	entries, err := helper.store.List()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.ID == id {
			return &entry, nil
		}
	}
	return nil, nil
}

func (helper Helper) removalInspect(ctx context.Context, identity linuxhost.PKExecIdentity, input io.Reader) (RemovalInspectionResponse, error) {
	var request RemovalInspectionRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return RemovalInspectionResponse{}, err
	}
	lock, err := helper.operationLocks.Shared()
	if err != nil {
		return RemovalInspectionResponse{}, err
	}
	scope, err := helper.removalScope(ctx, identity, request)
	if err = closeLockWithError(lock, err, "workspace operations"); err != nil {
		return RemovalInspectionResponse{}, err
	}
	return RemovalInspectionResponse{OK: true, Preview: scope.preview()}, nil
}

func (coordinator Coordinator) executeRemovalInspect(ctx context.Context, input io.Reader) (RemovalInspectionResponse, error) {
	var request RemovalInspectionRequest
	if err := strictjson.Decode(input, &request); err != nil {
		return RemovalInspectionResponse{}, err
	}
	var response RemovalInspectionResponse
	err := coordinator.privileged.invoke(ctx, "removal-inspect", request, &response)
	return response, err
}

func (helper Helper) executeRemoval(ctx context.Context, identity linuxhost.PKExecIdentity, request RemovalInspectionRequest, expected string) (RemovalResponse, error) {
	if expected == "" {
		return RemovalResponse{}, errors.New("inspect removal and confirm the current scope before deleting")
	}
	lock, err := helper.operationLocks.Exclusive()
	if err != nil {
		return RemovalResponse{}, err
	}
	response, operationErr := helper.confirmedRemoval(ctx, identity, request, expected)
	if err = closeLockWithError(lock, operationErr, "workspace operations"); err != nil {
		response.OK = false
		response.Problem = err.Error()
	}
	return response, nil
}

func (helper Helper) confirmedRemoval(ctx context.Context, identity linuxhost.PKExecIdentity, request RemovalInspectionRequest, expected string) (RemovalResponse, error) {
	if request.Action != "remove" {
		return helper.removeWithCurrentScope(ctx, identity, request, expected, nil)
	}
	locked, err := helper.store.Lock()
	if err != nil {
		return emptyRemovalResponse(), err
	}
	response, err := helper.removeWithCurrentScope(ctx, identity, request, expected, locked)
	return response, errors.Join(err, locked.Close())
}

func emptyRemovalResponse() RemovalResponse {
	return RemovalResponse{Result: linuxhost.DeletionResult{Removed: []string{}, NotAttempted: []string{}}, Catalog: "unchanged"}
}

func (helper Helper) removeWithCurrentScope(ctx context.Context, identity linuxhost.PKExecIdentity, request RemovalInspectionRequest, expected string, locked *catalog.LockedStore) (RemovalResponse, error) {
	scope, err := helper.removalScope(ctx, identity, request)
	if err != nil {
		return emptyRemovalResponse(), err
	}
	if scope.preview().Revision != expected {
		return emptyRemovalResponse(), errors.New("removal scope changed; inspect again and confirm the new selection")
	}
	if request.Action == "remove" && scope.Project == nil && len(scope.Accounts) != 0 {
		return emptyRemovalResponse(), errors.New("project is no longer catalogued; inspect orphaned accounts before cleanup")
	}
	return helper.removeSelectedAccounts(ctx, locked, scope), nil
}

func (helper Helper) removeSelectedAccounts(ctx context.Context, locked *catalog.LockedStore, scope removalScope) RemovalResponse {
	response := RemovalResponse{Result: linuxhost.DeleteAccounts(ctx, helper.deletion, scope.Accounts), Catalog: "unchanged"}
	if scope.Action == "remove" {
		response.Catalog = "not_attempted"
	}
	if response.Result.Uncertain != "" {
		return response
	}
	if scope.Project != nil {
		if err := locked.Remove(scope.Target); err != nil {
			response.Catalog, response.Problem = "uncertain", err.Error()
			return response
		}
		response.Catalog = "removed"
	}
	response.OK = true
	return response
}
