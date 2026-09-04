package acceptance

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
)

const (
	externalGitFixtureUsername   = "soda-git-fixture"
	externalGitFixtureProjectID  = "external-ssh"
	externalGitFixtureRepository = "/home/soda-git-fixture/external-ssh.git"
	externalGitFixtureURL        = "ssh://soda-git-fixture@127.0.0.1/home/soda-git-fixture/external-ssh.git"
)

const createExternalGitFixtureScript = `username=soda-git-fixture
home=/home/soda-git-fixture
repository="$home/external-ssh.git"
seed="$home/seed"
! getent passwd "$username" >/dev/null
! getent group "$username" >/dev/null
test ! -e "$home"
test -x /usr/bin/git-shell
created=0
cleanup_fixture() {
  if test "$created" -eq 1; then
    /usr/sbin/userdel --remove -- "$username" || true
    ! getent group "$username" >/dev/null || /usr/sbin/groupdel -- "$username" || true
  fi
}
trap cleanup_fixture EXIT
/usr/sbin/useradd --create-home --user-group --shell /usr/bin/git-shell --home-dir "$home" -- "$username"
created=1
/usr/bin/head --bytes=48 /dev/urandom | /usr/bin/base64 | /usr/bin/passwd --stdin -- "$username" >/dev/null
password_field=$(getent shadow "$username" | cut -d: -f2)
test -n "$password_field"
case "$password_field" in
  '!'*|'*') exit 1 ;;
esac
/usr/sbin/runuser --user "$username" -- /usr/bin/git init --bare --initial-branch=main -- "$repository"
/usr/sbin/runuser --user "$username" -- /usr/bin/git init --initial-branch=main -- "$seed"
printf 'external-ssh-fixture\n' | /usr/sbin/runuser --user "$username" -- /usr/bin/tee -- "$seed/fixture.txt" >/dev/null
/usr/sbin/runuser --user "$username" -- /usr/bin/git -C "$seed" config user.name 'Soda OS acceptance'
/usr/sbin/runuser --user "$username" -- /usr/bin/git -C "$seed" config user.email 'acceptance@localhost'
/usr/sbin/runuser --user "$username" -- /usr/bin/git -C "$seed" add -- fixture.txt
/usr/sbin/runuser --user "$username" -- /usr/bin/git -C "$seed" commit --message 'seed external SSH fixture'
/usr/sbin/runuser --user "$username" -- /usr/bin/git -C "$seed" remote add origin "$repository"
/usr/sbin/runuser --user "$username" -- /usr/bin/git -C "$seed" push --set-upstream origin main
/usr/sbin/runuser --user "$username" -- /usr/bin/rm --recursive --force -- "$seed"
test "$(/usr/sbin/runuser --user "$username" -- /usr/bin/git --git-dir="$repository" rev-parse --is-bare-repository)" = true
test "$(/usr/sbin/runuser --user "$username" -- /usr/bin/git --git-dir="$repository" show main:fixture.txt)" = external-ssh-fixture
trap - EXIT
printf 'local-guest-external-ssh-fixture=ready\n'
`

const teardownExternalGitFixtureScript = `username=soda-git-fixture
home=/home/soda-git-fixture
if entry=$(getent passwd "$username"); then
  test "$(cut -d: -f6 <<<"$entry")" = "$home"
  /usr/sbin/userdel --remove -- "$username"
fi
if getent group "$username" >/dev/null; then
  /usr/sbin/groupdel -- "$username"
fi
! getent passwd "$username" >/dev/null
! getent group "$username" >/dev/null
test ! -e "$home"
printf 'local-guest-external-ssh-fixture=removed\n'
`

func (state *runnerState) verifyExternalSSHRepository(ctx context.Context, scenario *scenarioState) error {
	if err := scenario.remote.Sudo(ctx, scenario.password, createExternalGitFixtureScript, "product/local-guest-external-ssh-fixture-create"); err != nil {
		return err
	}
	scenarioErr := state.exerciseExternalSSHRepository(ctx, scenario)
	teardownErr := scenario.remote.Sudo(ctx, scenario.password, teardownExternalGitFixtureScript, "product/local-guest-external-ssh-fixture-teardown")
	return errors.Join(scenarioErr, teardownErr)
}

func (state *runnerState) exerciseExternalSSHRepository(ctx context.Context, scenario *scenarioState) error {
	workspace, err := state.setupExternalSSHWorkspace(ctx, scenario)
	if err != nil {
		return err
	}
	return state.removeExternalSSHProject(ctx, scenario, workspace.WorkspaceUsername)
}

func (state *runnerState) setupExternalSSHWorkspace(ctx context.Context, scenario *scenarioState) (projectResponse, error) {
	payload := map[string]any{
		"id": externalGitFixtureProjectID, "display_name": "Local guest external SSH fixture", "canonical_url": externalGitFixtureURL,
	}
	if _, err := state.projectCall(ctx, scenario.remote, "add-existing", payload, "product/external-ssh-add-existing"); err != nil {
		return projectResponse{}, err
	}
	if err := state.requireWorkspaceAbsent(ctx, scenario.remote, externalGitFixtureProjectID, "product/external-ssh-no-workspace"); err != nil {
		return projectResponse{}, err
	}
	retained, err := state.requireRetainedWorkspace(ctx, scenario.remote, externalGitFixtureProjectID, "product/external-ssh-setup")
	if err != nil {
		return projectResponse{}, err
	}
	if err = requireExternalSSHAuthenticationFailure(retained.Diagnostic); err != nil {
		return projectResponse{}, err
	}
	if err = state.verifyRetainedExternalWorkspace(ctx, scenario, retained); err != nil {
		return projectResponse{}, err
	}
	if err = state.installExternalGitFixtureKey(ctx, scenario, retained.PublicKey); err != nil {
		return projectResponse{}, err
	}
	response, err := state.retryWorkspaceSetup(ctx, scenario.remote, externalGitFixtureProjectID, "product/external-ssh-setup")
	if err != nil {
		return projectResponse{}, err
	}
	if response.WorkspaceUsername != retained.Username {
		return projectResponse{}, errors.New("external SSH setup retry changed the retained workspace account")
	}
	return response, state.verifyExternalSSHClone(ctx, scenario, response.WorkspaceUsername)
}

func requireExternalSSHAuthenticationFailure(diagnostic []byte) error {
	if !bytes.Contains(diagnostic, []byte(externalGitFixtureUsername+"@127.0.0.1")) || !bytes.Contains(diagnostic, []byte("Permission denied")) {
		return errors.New("local guest external SSH fixture did not report its expected initial authentication failure")
	}
	return nil
}

func (state *runnerState) verifyRetainedExternalWorkspace(ctx context.Context, scenario *scenarioState, retained retainedWorkspace) error {
	expectedKey := string(retained.PublicKey)
	script := fmt.Sprintf(`workspace=%q
expected_key=%q
entry=$(getent passwd "$workspace")
home=$(cut -d: -f6 <<<"$entry")
test -d "$home"
test -s "$home/.ssh/id_ed25519_soda"
test -s "$home/.ssh/id_ed25519_soda.pub"
actual_key=$(awk '{print $1 " " $2}' "$home/.ssh/id_ed25519_soda.pub")
test "$actual_key" = "$expected_key"
`, retained.Username, expectedKey)
	return scenario.remote.Sudo(ctx, scenario.password, script, "product/external-ssh-retained-account-key")
}

func (state *runnerState) installExternalGitFixtureKey(ctx context.Context, scenario *scenarioState, publicKey []byte) error {
	key64 := base64.StdEncoding.EncodeToString(publicKey)
	script := fmt.Sprintf(`username=%q
expected_home=/home/soda-git-fixture
entry=$(getent passwd "$username")
home=$(cut -d: -f6 <<<"$entry")
shell=$(cut -d: -f7 <<<"$entry")
test "$home" = "$expected_home"
test "$shell" = /usr/bin/git-shell
/usr/bin/install -d -m 0700 -o "$username" -g "$username" "$home/.ssh"
printf '%%s\n' %q | /usr/bin/base64 --decode >"$home/.ssh/authorized_keys"
/usr/bin/chown "$username:$username" "$home/.ssh/authorized_keys"
/usr/bin/chmod 0600 "$home/.ssh/authorized_keys"
/usr/sbin/restorecon -RF "$home/.ssh"
`, externalGitFixtureUsername, key64)
	return scenario.remote.Sudo(ctx, scenario.password, script, "product/external-ssh-native-key-install")
}

func (state *runnerState) verifyExternalSSHClone(ctx context.Context, scenario *scenarioState, workspaceUsername string) error {
	workspace := scenario.remote.As(workspaceUsername, state.paths.adminKey)
	script := fmt.Sprintf(`set -euo pipefail
repository="$HOME/Projects/%s"
test -d "$repository/.git"
test "$(git -C "$repository" remote get-url origin)" = %q
test "$(cat "$repository/fixture.txt")" = external-ssh-fixture
test "$(git -C "$repository" rev-parse --is-inside-work-tree)" = true
`, externalGitFixtureProjectID, externalGitFixtureURL)
	return workspace.Capture(ctx, "product/external-ssh-complete-clone", []byte(script), "/bin/bash", "-s")
}

func (state *runnerState) removeExternalSSHProject(ctx context.Context, scenario *scenarioState, workspaceUsername string) error {
	if _, err := state.projectCall(ctx, scenario.remote, "remove", map[string]any{"id": externalGitFixtureProjectID}, "product/external-ssh-project-remove"); err != nil {
		return err
	}
	if err := state.requireProjectAbsent(ctx, scenario.remote, externalGitFixtureProjectID, "product/external-ssh-catalog-removed"); err != nil {
		return err
	}
	script := fmt.Sprintf(`workspace=%q
fixture_user=%q
fixture_repository=%q
! getent passwd "$workspace" >/dev/null
getent passwd "$fixture_user" >/dev/null
test "$(/usr/sbin/runuser --user "$fixture_user" -- /usr/bin/git --git-dir="$fixture_repository" rev-parse --is-bare-repository)" = true
test "$(/usr/sbin/runuser --user "$fixture_user" -- /usr/bin/git --git-dir="$fixture_repository" show main:fixture.txt)" = external-ssh-fixture
`, workspaceUsername, externalGitFixtureUsername, externalGitFixtureRepository)
	return scenario.remote.Sudo(ctx, scenario.password, script, "product/external-ssh-canonical-repository-preserved")
}

func (state *runnerState) requireProjectAbsent(ctx context.Context, remote Remote, projectID, evidence string) error {
	projects, err := state.catalogProjects(ctx, remote, evidence)
	if err != nil {
		return err
	}
	for _, project := range projects {
		if project.ID == projectID {
			return fmt.Errorf("catalog still contains removed project %s", projectID)
		}
	}
	return nil
}
