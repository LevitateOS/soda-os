package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

type projectResponse struct {
	OK                bool          `json:"ok"`
	WorkspaceUsername string        `json:"workspace_username"`
	Project           projectRecord `json:"project"`
	Preview           struct {
		Revision string `json:"revision"`
	} `json:"preview"`
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

func editCatalogMetadata(ctx context.Context, alice, bob Remote) error {
	canonicalURL, err := rejectCatalogURLEdit(ctx, alice)
	if err != nil {
		return err
	}
	return verifyCatalogMetadataEdit(ctx, alice, bob, canonicalURL)
}

func rejectCatalogURLEdit(ctx context.Context, alice Remote) (string, error) {
	projects, err := catalogProjects(ctx, alice, "seed/catalog-before-edit")
	if err != nil {
		return "", err
	}
	kept, err := catalogProject(projects, "kept")
	if err != nil {
		return "", err
	}
	replacementURL := "git@git.example.test:team/replacement.git"
	injected := map[string]any{"id": "kept", "display_name": "Kept project", "canonical_url": replacementURL, "team": "web"}
	result, err := invokeProject(ctx, alice, "edit", injected, "seed/catalog-url-edit-rejected")
	if err != nil {
		return "", err
	}
	if err = requireProjectRejection(result, `must not include "canonical_url"`); err != nil {
		return "", err
	}
	projects, err = catalogProjects(ctx, alice, "seed/catalog-after-url-edit-rejection")
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

func verifyCatalogMetadataEdit(ctx context.Context, alice, bob Remote, canonicalURL string) error {
	payload := map[string]any{"id": "kept", "display_name": "Kept project", "team": "web", "future": map[string]any{"shape": true}}
	if _, err := projectCall(ctx, alice, "edit", payload, "seed/catalog-edit"); err != nil {
		return err
	}
	projects, err := catalogProjects(ctx, bob, "seed/catalog-metadata")
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

func catalogProjects(ctx context.Context, remote Remote, evidence string) ([]projectRecord, error) {
	output, err := remote.CaptureOutput(ctx, evidence, []byte("{}\n"), "/usr/libexec/soda/soda-projects", "list")
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

func createCatalogedForgejoProject(ctx context.Context, person personFixture, project forgejoProject) (projectResponse, error) {
	canonicalURL, err := createNativeForgejoRepository(ctx, person, project.ID, project.Evidence+"-forgejo")
	if err != nil {
		return projectResponse{}, err
	}
	payload := map[string]any{"id": project.ID, "display_name": project.Name, "canonical_url": canonicalURL}
	response, err := projectCall(ctx, person.Remote, "add-existing", payload, project.Evidence+"-catalog")
	if err != nil {
		return projectResponse{}, err
	}
	if err = requireWorkspaceAbsent(ctx, person.Remote, project.ID, project.Evidence+"-catalog-no-workspace"); err != nil {
		return projectResponse{}, err
	}
	return response, nil
}

func createNativeForgejoRepository(ctx context.Context, person personFixture, id, evidence string) (string, error) {
	config := fmt.Sprintf("user = %s\nsilent\nshow-error\nfail-with-body\nurl = %s\n", curlConfigQuote(person.Remote.Username+":"+string(bytes.TrimSpace(person.ForgejoPassword))), curlConfigQuote(forgejoLoopbackEndpoint+"/api/v1/user/repos"))
	payload, err := json.Marshal(map[string]any{"name": id, "auto_init": false, "default_branch": "main"})
	if err != nil {
		return "", err
	}
	output, err := person.Remote.CaptureOutput(ctx, evidence, []byte(config), "curl", "--config", "-", "--json", string(payload))
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

func invokeProject(ctx context.Context, remote Remote, action string, payload any, evidence string) (CommandResult, error) {
	contents, err := json.Marshal(payload)
	if err != nil {
		return CommandResult{}, err
	}
	return remote.Exchange(ctx, evidence, append(contents, '\n'), "/usr/libexec/soda/soda-projects", action)
}

// soda-projects reports product rejection with exit 1 and a specific diagnostic.
// SSH failures, a missing executable, and unrelated product errors are not proof.
func requireProjectRejection(result CommandResult, diagnostic string) error {
	var exit *exec.ExitError
	if !errors.As(result.Err, &exit) || exit.ExitCode() != 1 || !bytes.Contains(result.Stderr, []byte(diagnostic)) {
		return fmt.Errorf("expected Projects rejection %q: %w", diagnostic, errors.Join(errors.New("unexpected command outcome"), result.Err))
	}
	return nil
}

func projectCall(ctx context.Context, remote Remote, action string, payload any, evidence string) (projectResponse, error) {
	result, err := invokeProject(ctx, remote, action, payload, evidence)
	if err = errors.Join(result.Err, err); err != nil {
		return projectResponse{}, err
	}
	var response projectResponse
	if err = json.Unmarshal(result.Stdout, &response); err != nil {
		return projectResponse{}, fmt.Errorf("decode %s response: %w", action, err)
	}
	if !response.OK {
		return projectResponse{}, fmt.Errorf("%s did not report success", action)
	}
	return response, nil
}

func projectRemoval(ctx context.Context, remote Remote, action, target, evidence string) (projectResponse, error) {
	inspected, err := projectCall(ctx, remote, "removal-inspect", map[string]string{"action": action, "target": target}, evidence+"-inspect")
	if err != nil {
		return projectResponse{}, err
	}
	if len(inspected.Preview.Revision) != 64 {
		return projectResponse{}, errors.New("removal inspection has no scope revision")
	}
	payload := map[string]string{"id": target, "expected": inspected.Preview.Revision}
	if action == "delete-human" {
		delete(payload, "id")
		payload["username"] = target
	}
	return projectCall(ctx, remote, action, payload, evidence)
}

func setupWorkspace(ctx context.Context, person personFixture, projectID, evidence string) (workspaceFixture, error) {
	retained, err := requireRetainedWorkspace(ctx, person.Remote, projectID, evidence)
	if err != nil {
		return workspaceFixture{}, err
	}
	if err = registerForgejoKey(ctx, person, retained.PublicKey, evidence+"-register-key"); err != nil {
		return workspaceFixture{}, err
	}
	return retryWorkspaceSetup(ctx, person, projectID, evidence)
}

type retainedWorkspace struct {
	Username   string
	PublicKey  []byte
	Diagnostic []byte
}

func requireRetainedWorkspace(ctx context.Context, remote Remote, projectID, evidence string) (retainedWorkspace, error) {
	payload := map[string]any{"id": projectID}
	result, err := invokeProject(ctx, remote, "setup", payload, evidence+"-key-required")
	if err != nil {
		return retainedWorkspace{}, err
	}
	if result.Err == nil {
		return retainedWorkspace{}, errors.New("workspace setup completed before its outbound Git key was registered")
	}
	if err = requireProjectRejection(result, "retained"); err != nil {
		return retainedWorkspace{}, err
	}
	diagnostic := result.Stderr
	if err = validateRetainedWorkspaceDiagnostic(diagnostic); err != nil {
		return retainedWorkspace{}, err
	}
	project, err := workspaceRecord(ctx, remote, projectID, evidence+"-retained-account")
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

func validateRetainedWorkspaceDiagnostic(diagnostic []byte) error {
	if !bytes.Contains(diagnostic, []byte("retained")) || !bytes.Contains(diagnostic, []byte("retry")) {
		return errors.New("workspace setup failure did not report retained state and retry guidance")
	}
	return nil
}

func retryWorkspaceSetup(ctx context.Context, person personFixture, projectID, evidence string) (workspaceFixture, error) {
	response, err := projectCall(ctx, person.Remote, "setup", map[string]any{"id": projectID}, evidence+"-retry")
	if err != nil {
		return workspaceFixture{}, err
	}
	if err = requireWorkspaceExists(ctx, person.Remote, projectID, evidence+"-complete-account"); err != nil {
		return workspaceFixture{}, err
	}
	if response.WorkspaceUsername == "" {
		return workspaceFixture{}, errors.New("workspace setup returned no workspace username")
	}
	return workspaceFixture{Person: person, Remote: person.Remote.As(response.WorkspaceUsername, person.Remote.Key), ProjectID: projectID}, nil
}

func workspaceRecord(ctx context.Context, remote Remote, projectID, evidence string) (projectRecord, error) {
	projects, err := catalogProjects(ctx, remote, evidence)
	if err != nil {
		return projectRecord{}, err
	}
	return catalogProject(projects, projectID)
}

func requireWorkspaceExists(ctx context.Context, remote Remote, projectID, evidence string) error {
	project, err := workspaceRecord(ctx, remote, projectID, evidence)
	if err != nil {
		return err
	}
	if !project.WorkspaceExists {
		return fmt.Errorf("project %s did not report an existing workspace account", projectID)
	}
	return nil
}

func requireWorkspaceAbsent(ctx context.Context, remote Remote, projectID, evidence string) error {
	project, err := workspaceRecord(ctx, remote, projectID, evidence)
	if err != nil {
		return err
	}
	if project.WorkspaceExists {
		return fmt.Errorf("project %s unexpectedly reported an existing workspace account", projectID)
	}
	return nil
}
