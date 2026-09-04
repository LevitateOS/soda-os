package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type projectResponse struct {
	OK                bool          `json:"ok"`
	WorkspaceUsername string        `json:"workspace_username"`
	Project           projectRecord `json:"project"`
}

type projectRecord struct {
	ID                string         `json:"id"`
	DisplayName       string         `json:"display_name"`
	CanonicalURL      string         `json:"canonical_url"`
	Metadata          map[string]any `json:"catalog_metadata"`
	WorkspaceUsername string         `json:"workspace_username"`
	WorkspaceExists   bool           `json:"workspace_exists"`
}

type forgejoProject struct {
	ID       string
	Name     string
	Evidence string
}

func (state *runnerState) editCatalogMetadata(ctx context.Context, scenario *scenarioState) error {
	alice := scenario.remote.As("alice", state.personKeyPath("alice"))
	canonicalURL, err := state.rejectCatalogURLEdit(ctx, alice)
	if err != nil {
		return err
	}
	return state.verifyCatalogMetadataEdit(ctx, scenario, alice, canonicalURL)
}

func (state *runnerState) rejectCatalogURLEdit(ctx context.Context, alice Remote) (string, error) {
	projects, err := state.catalogProjects(ctx, alice, "seed/catalog-before-edit")
	if err != nil {
		return "", err
	}
	kept, err := catalogProject(projects, "kept")
	if err != nil {
		return "", err
	}
	replacementURL := "git@git.example.test:team/replacement.git"
	injected := map[string]any{"id": "kept", "display_name": "Kept project", "canonical_url": replacementURL, "team": "web"}
	if _, err = state.projectCall(ctx, alice, "edit", injected, "seed/catalog-url-edit-rejected"); err == nil {
		return "", errors.New("Projects accepted a canonical URL in an edit request")
	}
	projects, err = state.catalogProjects(ctx, alice, "seed/catalog-after-url-edit-rejection")
	if err != nil {
		return "", err
	}
	afterRejection, err := catalogProject(projects, "kept")
	if err != nil {
		return "", err
	}
	if afterRejection.CanonicalURL != kept.CanonicalURL {
		return "", errors.New("rejected edit changed the canonical project URL")
	}
	return kept.CanonicalURL, nil
}

func (state *runnerState) verifyCatalogMetadataEdit(ctx context.Context, scenario *scenarioState, alice Remote, canonicalURL string) error {
	payload := map[string]any{"id": "kept", "display_name": "Kept project", "team": "web", "future": map[string]any{"shape": true}}
	if _, err := state.projectCall(ctx, alice, "edit", payload, "seed/catalog-edit"); err != nil {
		return err
	}
	bob := scenario.remote.As("bob", state.personKeyPath("bob"))
	projects, err := state.catalogProjects(ctx, bob, "seed/catalog-metadata")
	if err != nil {
		return err
	}
	kept, err := catalogProject(projects, "kept")
	if err != nil {
		return err
	}
	future, ok := kept.Metadata["future"].(map[string]any)
	if !ok || future["shape"] != true {
		return errors.New("arbitrary catalog metadata did not round-trip")
	}
	if kept.CanonicalURL != canonicalURL {
		return errors.New("metadata edit changed the canonical project URL")
	}
	return nil
}

func (state *runnerState) catalogProjects(ctx context.Context, remote Remote, evidence string) ([]projectRecord, error) {
	output, err := remote.Exchange(ctx, evidence, []byte("{}\n"), "/usr/libexec/soda/soda-projects", "list")
	if err != nil {
		return nil, err
	}
	var response struct {
		Projects []projectRecord `json:"projects"`
	}
	if err = json.Unmarshal(output, &response); err != nil {
		return nil, err
	}
	return response.Projects, nil
}

func catalogProject(projects []projectRecord, id string) (projectRecord, error) {
	for _, project := range projects {
		if project.ID == id {
			return project, nil
		}
	}
	return projectRecord{}, fmt.Errorf("catalog does not contain project %s", id)
}

func (state *runnerState) createCatalogedForgejoProject(ctx context.Context, remote Remote, password []byte, project forgejoProject) (projectResponse, error) {
	canonicalURL, err := state.createNativeForgejoRepository(ctx, remote, password, project.ID, project.Evidence+"-forgejo")
	if err != nil {
		return projectResponse{}, err
	}
	payload := map[string]any{"id": project.ID, "display_name": project.Name, "canonical_url": canonicalURL}
	response, err := state.projectCall(ctx, remote, "add-existing", payload, project.Evidence+"-catalog")
	if err != nil {
		return projectResponse{}, err
	}
	if err = state.requireWorkspaceAbsent(ctx, remote, project.ID, project.Evidence+"-catalog-no-workspace"); err != nil {
		return projectResponse{}, err
	}
	return response, nil
}

func (state *runnerState) createNativeForgejoRepository(ctx context.Context, remote Remote, password []byte, id, evidence string) (string, error) {
	config := fmt.Sprintf("user = %s\nsilent\nshow-error\nfail-with-body\nurl = %s\n", curlConfigQuote(remote.Username+":"+string(bytes.TrimSpace(password))), curlConfigQuote(forgejoLoopbackEndpoint+"/api/v1/user/repos"))
	payload, err := json.Marshal(map[string]any{"name": id, "auto_init": false})
	if err != nil {
		return "", err
	}
	output, err := remote.Exchange(ctx, evidence, []byte(config), "curl", "--config", "-", "--json", string(payload))
	if err != nil {
		return "", err
	}
	var repository struct {
		SSHURL string `json:"ssh_url"`
	}
	if err = json.Unmarshal(output, &repository); err != nil {
		return "", fmt.Errorf("decode native Forgejo repository: %w", err)
	}
	if repository.SSHURL == "" {
		return "", errors.New("native Forgejo repository response has no SSH clone URL")
	}
	return repository.SSHURL, nil
}

func (state *runnerState) projectCall(ctx context.Context, remote Remote, action string, payload any, evidence string) (projectResponse, error) {
	contents, err := json.Marshal(payload)
	if err != nil {
		return projectResponse{}, err
	}
	contents = append(contents, '\n')
	output, err := remote.Exchange(ctx, evidence, contents, "/usr/libexec/soda/soda-projects", action)
	if err != nil {
		return projectResponse{}, err
	}
	var response projectResponse
	if err = json.Unmarshal(output, &response); err != nil {
		return projectResponse{}, fmt.Errorf("decode %s response: %w", action, err)
	}
	if !response.OK {
		return projectResponse{}, fmt.Errorf("%s did not report success", action)
	}
	return response, nil
}

func (state *runnerState) setupWorkspace(ctx context.Context, remote Remote, password []byte, projectID, evidence string) (projectResponse, error) {
	retained, err := state.requireRetainedWorkspace(ctx, remote, projectID, evidence)
	if err != nil {
		return projectResponse{}, err
	}
	if err = state.registerForgejoKey(ctx, remote, password, retained.PublicKey, evidence+"-register-key"); err != nil {
		return projectResponse{}, err
	}
	return state.retryWorkspaceSetup(ctx, remote, projectID, evidence)
}

type retainedWorkspace struct {
	Username   string
	PublicKey  []byte
	Diagnostic []byte
}

func (state *runnerState) requireRetainedWorkspace(ctx context.Context, remote Remote, projectID, evidence string) (retainedWorkspace, error) {
	payload := map[string]any{"id": projectID}
	if _, err := state.projectCall(ctx, remote, "setup", payload, evidence+"-key-required"); err == nil {
		return retainedWorkspace{}, errors.New("workspace setup completed before its outbound Git key was registered")
	}
	diagnostic, err := workspaceSetupDiagnostic(remote, evidence)
	if err != nil {
		return retainedWorkspace{}, err
	}
	if err = validateRetainedWorkspaceDiagnostic(diagnostic); err != nil {
		return retainedWorkspace{}, err
	}
	project, err := state.workspaceRecord(ctx, remote, projectID, evidence+"-retained-account")
	if err != nil {
		return retainedWorkspace{}, err
	}
	if !project.WorkspaceExists || project.WorkspaceUsername == "" {
		return retainedWorkspace{}, fmt.Errorf("project %s did not report its retained workspace account", projectID)
	}
	publicKey, err := workspacePublicKeyFromDiagnostic(diagnostic)
	if err != nil {
		return retainedWorkspace{}, err
	}
	return retainedWorkspace{Username: project.WorkspaceUsername, PublicKey: publicKey, Diagnostic: diagnostic}, nil
}

func workspaceSetupDiagnostic(remote Remote, evidence string) ([]byte, error) {
	diagnosticPath, err := remote.Evidence.path(evidence + "-key-required.stderr")
	if err != nil {
		return nil, err
	}
	diagnostic, err := os.ReadFile(diagnosticPath)
	if err != nil {
		return nil, fmt.Errorf("read retained workspace-key diagnostic: %w", err)
	}
	return diagnostic, nil
}

func validateRetainedWorkspaceDiagnostic(diagnostic []byte) error {
	if !bytes.Contains(diagnostic, []byte("retained")) || !bytes.Contains(diagnostic, []byte("retry")) {
		return errors.New("workspace setup failure did not report retained state and retry guidance")
	}
	return nil
}

func (state *runnerState) retryWorkspaceSetup(ctx context.Context, remote Remote, projectID, evidence string) (projectResponse, error) {
	payload := map[string]any{"id": projectID}
	response, err := state.projectCall(ctx, remote, "setup", payload, evidence+"-retry")
	if err != nil {
		return projectResponse{}, err
	}
	if err = state.requireWorkspaceExists(ctx, remote, projectID, evidence+"-complete-account"); err != nil {
		return projectResponse{}, err
	}
	return response, nil
}

func (state *runnerState) workspaceRecord(ctx context.Context, remote Remote, projectID, evidence string) (projectRecord, error) {
	projects, err := state.catalogProjects(ctx, remote, evidence)
	if err != nil {
		return projectRecord{}, err
	}
	return catalogProject(projects, projectID)
}

func (state *runnerState) requireWorkspaceExists(ctx context.Context, remote Remote, projectID, evidence string) error {
	project, err := state.workspaceRecord(ctx, remote, projectID, evidence)
	if err != nil {
		return err
	}
	if !project.WorkspaceExists {
		return fmt.Errorf("project %s did not report an existing workspace account", projectID)
	}
	return nil
}

func (state *runnerState) requireWorkspaceAbsent(ctx context.Context, remote Remote, projectID, evidence string) error {
	project, err := state.workspaceRecord(ctx, remote, projectID, evidence)
	if err != nil {
		return err
	}
	if project.WorkspaceExists {
		return fmt.Errorf("project %s unexpectedly reported an existing workspace account", projectID)
	}
	return nil
}
