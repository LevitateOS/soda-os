package projects

import (
	"encoding/json"

	"github.com/LevitateOS/soda-os/internal/projects/catalog"
)

type EmptyRequest struct{}

// The catalog owns both the complete add request and the URL-preserving edit
// request. The public coordinator and privileged helper deliberately exchange
// those same values without a second Projects representation.
type AddExistingRequest = catalog.Entry

type EditRequest = catalog.Edit

type SetupRequest struct {
	ID string `json:"id"`
}

type ProjectRequest struct {
	ID string `json:"id"`
}

type DeleteHumanRequest struct {
	Username string `json:"username"`
}

type ProjectView struct {
	ID                string                     `json:"id"`
	DisplayName       string                     `json:"display_name"`
	CanonicalURL      string                     `json:"canonical_url"`
	CatalogMetadata   map[string]json.RawMessage `json:"catalog_metadata"`
	WorkspaceUsername string                     `json:"workspace_username"`
	WorkspaceExists   bool                       `json:"workspace_exists"`
}

func newProjectView(entry catalog.Entry, workspaceUsername string, workspaceExists bool) ProjectView {
	metadata := entry.Additional
	if metadata == nil {
		metadata = map[string]json.RawMessage{}
	}
	return ProjectView{
		ID:                entry.ID,
		DisplayName:       entry.DisplayName,
		CanonicalURL:      entry.CanonicalURL,
		CatalogMetadata:   metadata,
		WorkspaceUsername: workspaceUsername,
		WorkspaceExists:   workspaceExists,
	}
}

type CurrentUserView struct {
	Username      string `json:"username"`
	Administrator bool   `json:"administrator"`
}

type ListResponse struct {
	Projects    []ProjectView   `json:"projects"`
	CurrentUser CurrentUserView `json:"current_user"`
}

type ProjectMutationResponse struct {
	OK      bool        `json:"ok"`
	Project ProjectView `json:"project"`
}

type SetupResponse struct {
	OK                bool   `json:"ok"`
	WorkspaceUsername string `json:"workspace_username"`
}

type SuccessResponse struct {
	OK bool `json:"ok"`
}

type WorkspacePreparationResponse struct {
	OK                 bool   `json:"ok"`
	WorkspaceUsername  string `json:"workspace_username"`
	WorkspacePublicKey string `json:"workspace_public_key"`
}

type WorkspacePublicationResponse struct {
	OK                bool   `json:"ok"`
	WorkspaceUsername string `json:"workspace_username"`
}

type HelperCatalogRequest = catalog.Entry

type HelperEditRequest = catalog.Edit

type HelperWorkspaceRequest struct {
	ID           string `json:"id"`
	CanonicalURL string `json:"canonical_url"`
}

type HelperHumanRequest struct {
	Username string `json:"username"`
}
