package acceptance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type projectResponse struct {
	OK                bool          `json:"ok"`
	WorkspaceUsername string        `json:"workspace_username"`
	Project           projectRecord `json:"project"`
}

type projectRecord struct {
	ID           string         `json:"id"`
	DisplayName  string         `json:"display_name"`
	CanonicalURL string         `json:"canonical_url"`
	Metadata     map[string]any `json:"catalog_metadata"`
}

type forgejoProject struct {
	ID       string
	Name     string
	Evidence string
}

type scenarioState struct {
	remote     Remote
	password   []byte
	adminSpace string
	aliceSpace string
	bobSpace   string
}

func (state *runnerState) exerciseInstalledSystem(ctx context.Context, remote *Remote, vm **VM) error {
	password, err := os.ReadFile(state.paths.password)
	if err != nil {
		return err
	}
	scenario := scenarioState{remote: *remote, password: password}
	if err = state.captureCore(ctx, scenario.remote, "iso"); err != nil {
		return fmt.Errorf("installed product boundaries: %w", err)
	}
	if err = state.seedPreservationState(ctx, &scenario); err != nil {
		return fmt.Errorf("seed update and fallback state: %w", err)
	}
	if err = state.exerciseFallback(ctx, &scenario, vm); err != nil {
		return fmt.Errorf("manual update and fallback: %w", err)
	}
	*remote = scenario.remote
	if err = state.exerciseProductScenarios(ctx, &scenario); err != nil {
		return fmt.Errorf("product scenarios: %w", err)
	}
	if err = state.captureCore(ctx, scenario.remote, "final"); err != nil {
		return fmt.Errorf("final product capture: %w", err)
	}
	if state.logout == nil {
		return errors.New("installed Tailnet cleanup was not registered")
	}
	if err = state.logout(ctx); err != nil {
		return fmt.Errorf("revoke guest Tailnet enrollment: %w", err)
	}
	return (*vm).PowerDown(ctx)
}

func (state *runnerState) captureCore(ctx context.Context, remote Remote, prefix string) error {
	if err := remote.Capture(ctx, prefix+"/core", []byte(coreGuestChecks), "/bin/bash", "-s"); err != nil {
		return err
	}
	if err := remote.Capture(ctx, prefix+"/tailscale-access", []byte(tailscaleAccessCheck), "/bin/bash", "-s"); err != nil {
		return err
	}
	return remote.Sudo(ctx, state.secret("administrator-password"), stableManifestScript, prefix+"/system-manifest")
}

func (state *runnerState) runQCOW2Checks(ctx context.Context, remote Remote) error {
	if err := remote.Capture(ctx, "qcow2/core", []byte(coreGuestChecks), "/bin/bash", "-s"); err != nil {
		return err
	}
	if err := remote.Capture(ctx, "qcow2/setup", []byte(qcow2GuestChecks), "/bin/bash", "-s"); err != nil {
		return err
	}
	return remote.Capture(ctx, "qcow2/local-access", []byte(localAccessCheck), "/bin/bash", "-s")
}

func (state *runnerState) seedPreservationState(ctx context.Context, scenario *scenarioState) error {
	response, err := state.createCatalogedForgejoProject(ctx, scenario.remote, scenario.password, forgejoProject{"kept", "Kept project", "seed/kept-create"})
	if err != nil {
		return err
	}
	setup := map[string]any{"id": "kept", "forgejo_password": string(bytes.TrimSpace(scenario.password))}
	response, err = state.projectCall(ctx, scenario.remote, "setup", setup, "seed/admin-setup")
	if err != nil {
		return err
	}
	scenario.adminSpace = response.WorkspaceUsername
	for _, username := range []string{"alice", "bob"} {
		if err = state.addNativePerson(ctx, scenario.remote, username, scenario.password, "seed/"+username+"-add"); err != nil {
			return err
		}
		person := scenario.remote.As(username, state.personKeyPath(username))
		response, err = state.projectCall(ctx, person, "setup", map[string]any{"id": "kept", "forgejo_password": string(bytes.TrimSpace(scenario.password))}, "seed/"+username+"-setup")
		if err != nil {
			return err
		}
		if username == "alice" {
			scenario.aliceSpace = response.WorkspaceUsername
		} else {
			scenario.bobSpace = response.WorkspaceUsername
		}
	}
	if err = state.editCatalogMetadata(ctx, scenario); err != nil {
		return err
	}
	return state.seedWorkspaceFiles(ctx, scenario)
}

func (state *runnerState) editCatalogMetadata(ctx context.Context, scenario *scenarioState) error {
	alice := scenario.remote.As("alice", state.personKeyPath("alice"))
	projects, err := state.catalogProjects(ctx, alice, "seed/catalog-before-edit")
	if err != nil {
		return err
	}
	kept, err := catalogProject(projects, "kept")
	if err != nil {
		return err
	}
	payload := map[string]any{"id": "kept", "display_name": "Kept project", "canonical_url": kept.CanonicalURL, "team": "web", "future": map[string]any{"shape": true}}
	if _, err = state.projectCall(ctx, alice, "edit", payload, "seed/catalog-edit"); err != nil {
		return err
	}
	bob := scenario.remote.As("bob", state.personKeyPath("bob"))
	projects, err = state.catalogProjects(ctx, bob, "seed/catalog-metadata")
	if err != nil {
		return err
	}
	kept, err = catalogProject(projects, "kept")
	if err != nil {
		return err
	}
	future, ok := kept.Metadata["future"].(map[string]any)
	if !ok || future["shape"] != true {
		return errors.New("arbitrary catalog metadata did not round-trip")
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

func (state *runnerState) seedWorkspaceFiles(ctx context.Context, scenario *scenarioState) error {
	for label, workspace := range map[string]string{"admin": scenario.adminSpace, "alice": scenario.aliceSpace, "bob": scenario.bobSpace} {
		remote := scenario.remote.As(workspace, state.personKeyForLabel(label))
		script := "set -eu; printf '%s-private\\n' " + label + " >\"$HOME/Projects/kept/" + label + "-private.txt\"; printf 'preserved\\n' >\"$HOME/soda-acceptance-state.txt\""
		if err := remote.Capture(ctx, "seed/"+label+"-workspace-state", []byte(script), "/bin/bash", "-s"); err != nil {
			return err
		}
	}
	return scenario.remote.Sudo(ctx, scenario.password, workspaceCheckScript(state.options.Administrator.Username, "kept", scenario.adminSpace), "seed/workspace-boundary")
}

func (state *runnerState) createCatalogedForgejoProject(ctx context.Context, remote Remote, password []byte, project forgejoProject) (projectResponse, error) {
	canonicalURL, err := state.createNativeForgejoRepository(ctx, remote, password, project.ID, project.Evidence+"-forgejo")
	if err != nil {
		return projectResponse{}, err
	}
	payload := map[string]any{"id": project.ID, "display_name": project.Name, "canonical_url": canonicalURL}
	return state.projectCall(ctx, remote, "add-existing", payload, project.Evidence+"-catalog")
}

func (state *runnerState) createNativeForgejoRepository(ctx context.Context, remote Remote, password []byte, id, evidence string) (string, error) {
	endpoint, err := forgejoEndpoint(ctx, remote)
	if err != nil {
		return "", err
	}
	config := fmt.Sprintf("user = %s\nsilent\nshow-error\nfail-with-body\nurl = %s\n", curlConfigQuote(remote.Username+":"+string(bytes.TrimSpace(password))), curlConfigQuote(endpoint+"/api/v1/user/repos"))
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

func (state *runnerState) addNativePerson(ctx context.Context, remote Remote, username string, password []byte, evidence string) error {
	publicKey, err := state.ensurePersonKey(ctx, username)
	if err != nil {
		return err
	}
	password64 := base64.StdEncoding.EncodeToString(bytes.TrimSpace(password))
	key64 := base64.StdEncoding.EncodeToString(bytes.TrimSpace(publicKey))
	script := fmt.Sprintf(`username=%q
/usr/sbin/useradd --create-home --user-group --shell /bin/bash --home-dir "/home/$username" -- "$username"
printf '%%s' %q | base64 --decode | /usr/bin/passwd --stdin -- "$username"
/usr/bin/install -d -m 0700 -o "$username" -g "$username" "/home/$username/.ssh"
printf '%%s' %q | base64 --decode >"/home/$username/.ssh/authorized_keys"
/usr/bin/chown "$username:$username" "/home/$username/.ssh/authorized_keys"
/usr/bin/chmod 0600 "/home/$username/.ssh/authorized_keys"
/usr/sbin/restorecon -RF "/home/$username/.ssh"
`, username, password64, key64)
	if err = remote.Sudo(ctx, password, script, evidence+"-linux"); err != nil {
		return err
	}
	person := remote.As(username, state.personKeyPath(username))
	user, err := forgejoAuthenticatedUser(ctx, person, username, password)
	if err != nil {
		return err
	}
	if user.Login != username || user.IsAdmin {
		return fmt.Errorf("native Forgejo PAM created unexpected person %q", user.Login)
	}
	return nil
}

func (state *runnerState) ensurePersonKey(ctx context.Context, username string) ([]byte, error) {
	path := state.personKeyPath(username)
	if _, err := os.Stat(path); err == nil {
		return os.ReadFile(path + ".pub")
	}
	if err := RunCommand(ctx, CommandSpec{Name: "ssh-keygen", Args: []string{"-q", "-t", "ed25519", "-N", "", "-C", username + "@soda-acceptance", "-f", path}}); err != nil {
		return nil, err
	}
	privateKey, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state.secrets = append(state.secrets, Secret{Label: username + "-private-key", Value: privateKey})
	return os.ReadFile(path + ".pub")
}

func (state *runnerState) personKeyPath(username string) string {
	return filepath.Join(state.paths.people, username)
}

func (state *runnerState) personKeyForLabel(label string) string {
	if label == "admin" {
		return state.paths.adminKey
	}
	return state.personKeyPath(label)
}

func (state *runnerState) secret(label string) []byte {
	for _, secret := range state.secrets {
		if secret.Label == label {
			return secret.Value
		}
	}
	return nil
}

func workspaceCheckScript(primary, project, workspace string) string {
	return "exec /bin/bash -s -- " + primary + " " + project + " " + workspace + "\n" + workspaceBoundaryChecks
}

func waitBriefly(ctx context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
