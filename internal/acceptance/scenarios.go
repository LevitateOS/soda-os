package acceptance

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
	password := state.secret("administrator-password")
	if err := remote.Sudo(ctx, password, setupCompleteChecks, prefix+"/setup"); err != nil {
		return err
	}
	if err := remote.Capture(ctx, prefix+"/core", []byte(coreGuestChecks), "/bin/bash", "-s"); err != nil {
		return err
	}
	if err := remote.Sudo(ctx, password, tailscaleAccessCheck, prefix+"/tailscale-access"); err != nil {
		return err
	}
	return remote.Sudo(ctx, password, stableManifestScript, prefix+"/system-manifest")
}

func (state *runnerState) runQCOW2Checks(ctx context.Context, remote Remote) error {
	if err := remote.Capture(ctx, "qcow2/core", []byte(coreGuestChecks), "/bin/bash", "-s"); err != nil {
		return err
	}
	password := state.secret("administrator-password")
	if err := remote.Sudo(ctx, password, qcow2GuestChecks, "qcow2/setup"); err != nil {
		return err
	}
	if err := state.verifyLocalProjectsWithoutTailscale(ctx, remote); err != nil {
		return err
	}
	return remote.Sudo(ctx, password, localAccessCheck, "qcow2/local-access")
}

func (state *runnerState) verifyLocalProjectsWithoutTailscale(ctx context.Context, remote Remote) error {
	projects, err := state.catalogProjects(ctx, remote, "qcow2/projects-list-without-tailscale")
	if err != nil {
		return err
	}
	if len(projects) != 0 {
		return errors.New("reusable QCOW2 project catalog is not empty before local-only setup")
	}
	if _, err = state.createCatalogedForgejoProject(ctx, remote, state.secret("administrator-password"), forgejoProject{
		ID: "local-only", Name: "Local-only project", Evidence: "qcow2/local-project-create",
	}); err != nil {
		return err
	}
	response, err := state.setupWorkspace(ctx, remote, state.secret("administrator-password"), "local-only", "qcow2/local-project-setup")
	if err != nil {
		return err
	}
	if response.WorkspaceUsername == "" {
		return errors.New("local-only workspace setup returned no workspace username")
	}
	return remote.Sudo(ctx, state.secret("administrator-password"), `set -euo pipefail
status=$(/usr/libexec/soda/soda-setup status)
jq -e '(.tailscale_connected | not)' <<<"$status" >/dev/null
`, "qcow2/projects-setup-without-tailscale")
}

func (state *runnerState) seedPreservationState(ctx context.Context, scenario *scenarioState) error {
	response, err := state.createCatalogedForgejoProject(ctx, scenario.remote, scenario.password, forgejoProject{"kept", "Kept project", "seed/kept-create"})
	if err != nil {
		return err
	}
	response, err = state.setupWorkspace(ctx, scenario.remote, scenario.password, "kept", "seed/admin-setup")
	if err != nil {
		return err
	}
	scenario.adminSpace = response.WorkspaceUsername
	for _, username := range []string{"alice", "bob"} {
		if err = state.addNativePerson(ctx, scenario.remote, username, scenario.password, "seed/"+username+"-add"); err != nil {
			return err
		}
		person := scenario.remote.As(username, state.personKeyPath(username))
		response, err = state.setupWorkspace(ctx, person, scenario.password, "kept", "seed/"+username+"-setup")
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
