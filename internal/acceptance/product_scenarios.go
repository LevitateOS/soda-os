package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (state *runnerState) exerciseProductScenarios(ctx context.Context, scenario *scenarioState) error {
	steps := []func(context.Context, *scenarioState) error{
		state.verifyWorkspaceBoundaries,
		state.verifySSHTransports,
		state.verifyDevelopmentServer,
		state.verifyMiseOwnership,
		state.verifyWorkspaceRemoval,
		state.verifyCockpitAndRoles,
		state.verifyExternalSSHRepository,
		state.verifyProjectRemoval,
		state.verifyPartialPersonDeletion,
	}
	for _, step := range steps {
		if err := step(ctx, scenario); err != nil {
			return err
		}
	}
	return nil
}

func (state *runnerState) verifyWorkspaceBoundaries(ctx context.Context, scenario *scenarioState) error {
	identities := []struct{ label, primary, workspace string }{
		{"admin", state.options.Administrator.Username, scenario.adminSpace},
		{"alice", "alice", scenario.aliceSpace},
		{"bob", "bob", scenario.bobSpace},
	}
	for _, identity := range identities {
		script := workspaceCheckScript(identity.primary, "kept", identity.workspace)
		if err := scenario.remote.Sudo(ctx, scenario.password, script, "product/"+identity.label+"-workspace-boundary"); err != nil {
			return err
		}
	}
	if err := state.verifyIndependentWorkspaceUIDs(ctx, scenario); err != nil {
		return err
	}
	if err := state.verifyWorkspaceGitKeys(ctx, scenario); err != nil {
		return err
	}
	if err := state.verifyWorkspaceForgejoAbsence(ctx, scenario); err != nil {
		return err
	}
	return state.verifyOneTimeAuthorizedKeys(ctx, scenario)
}

func (state *runnerState) verifyIndependentWorkspaceUIDs(ctx context.Context, scenario *scenarioState) error {
	script := fmt.Sprintf("test \"$(printf '%%s\\n' $(id -u %s) $(id -u %s) $(id -u %s) | sort -u | wc -l)\" -eq 3", scenario.adminSpace, scenario.aliceSpace, scenario.bobSpace)
	return scenario.remote.Sudo(ctx, scenario.password, script, "product/workspace-uids")
}

func (state *runnerState) verifyWorkspaceForgejoAbsence(ctx context.Context, scenario *scenarioState) error {
	script := "set -eu; url=" + forgejoLoopbackEndpoint + "; for user in " + scenario.adminSpace + " " + scenario.aliceSpace + " " + scenario.bobSpace + "; do test \"$(curl --silent --output /dev/null --write-out '%{http_code}' \"$url/api/v1/users/$user\")\" = 404; done"
	return scenario.remote.Capture(ctx, "product/workspace-forgejo-absence", []byte(script), "/bin/bash", "-s")
}

func (state *runnerState) verifySSHTransports(ctx context.Context, scenario *scenarioState) error {
	workspace := scenario.remote.As(scenario.aliceSpace, state.personKeyPath("alice"))
	if err := workspace.Capture(ctx, "product/direct-command", nil, "id"); err != nil {
		return err
	}
	source := filepath.Join(state.evidence.Root, "product", "scp-input.txt")
	if err := state.evidence.Write("product/scp-input.txt", []byte("scp-product-evidence\n")); err != nil {
		return err
	}
	args := append(scpArgs(workspace), source, scenario.aliceSpace+"@"+workspace.Host+":scp-input.txt")
	if err := RunCommand(ctx, CommandSpec{Name: "scp", Args: args}); err != nil {
		return err
	}
	if err := workspace.Capture(ctx, "product/scp-content", []byte("test \"$(cat \"$HOME/scp-input.txt\")\" = scp-product-evidence\n"), "/bin/bash", "-s"); err != nil {
		return err
	}
	batch := []byte("pwd\nls -l scp-input.txt\nquit\n")
	output, err := CommandOutput(ctx, CommandSpec{Name: "sftp", Args: append([]string{"-q", "-b", "-"}, sftpArgs(workspace)...), Stdin: bytes.NewReader(batch)})
	if err != nil {
		return err
	}
	return state.evidence.Write("product/sftp.txt", output)
}

func (state *runnerState) verifyCockpitAndRoles(ctx context.Context, scenario *scenarioState) error {
	status, err := cockpitLoginStatus(ctx, scenario.remote, state.options.Administrator.Username, scenario.password)
	if err != nil || status != "200" {
		return fmt.Errorf("primary Cockpit authentication returned %s: %w", status, err)
	}
	status, err = cockpitLoginStatus(ctx, scenario.remote, scenario.adminSpace, []byte("locked-workspace-password"))
	if err != nil || status != "401" {
		return fmt.Errorf("workspace Cockpit authentication returned %s: %w", status, err)
	}
	if err = scenario.remote.Sudo(ctx, scenario.password, "/usr/sbin/usermod --append --groups wheel -- alice\n", "product/alice-wheel-promotion"); err != nil {
		return err
	}
	response, err := forgejoAuthenticatedUser(ctx, scenario.remote.As("alice", state.personKeyPath("alice")), "alice", scenario.password)
	if err != nil {
		return err
	}
	if response.Login != "alice" || response.IsAdmin {
		return errors.New("Linux wheel promotion changed native Forgejo administration")
	}
	return state.evidence.Write("product/cockpit-status.txt", []byte("primary=200\nworkspace=401\n"))
}

type forgejoUser struct {
	Login   string `json:"login"`
	IsAdmin bool   `json:"is_admin"`
}

func cockpitLoginStatus(ctx context.Context, remote Remote, username string, password []byte) (string, error) {
	config := fmt.Sprintf("user = %s\ninsecure\nsilent\nshow-error\noutput = \"/dev/null\"\nwrite-out = \"%%{http_code}\"\n", curlConfigQuote(username+":"+string(bytes.TrimSpace(password))))
	url := "https://" + urlHost(remote.Host) + ":" + strconv.Itoa(remote.CockpitPort) + "/cockpit/login"
	config += "url = " + curlConfigQuote(url) + "\n"
	output, err := CommandOutput(ctx, CommandSpec{Name: "curl", Args: []string{"--config", "-", "--request", "GET"}, Stdin: bytes.NewReader([]byte(config))})
	return strings.TrimSpace(string(output)), err
}

func forgejoAuthenticatedUser(ctx context.Context, remote Remote, username string, password []byte) (forgejoUser, error) {
	config := fmt.Sprintf("user = %s\nsilent\nshow-error\nfail-with-body\nurl = %s\n", curlConfigQuote(username+":"+string(bytes.TrimSpace(password))), curlConfigQuote(forgejoLoopbackEndpoint+"/api/v1/user"))
	output, err := remote.Exchange(ctx, "product/"+username+"-forgejo-user", []byte(config), "curl", "--config", "-")
	if err != nil {
		return forgejoUser{}, err
	}
	var user forgejoUser
	err = json.Unmarshal(output, &user)
	return user, err
}

func (state *runnerState) verifyDevelopmentServer(ctx context.Context, scenario *scenarioState) error {
	alice := scenario.remote.As(scenario.aliceSpace, state.personKeyPath("alice"))
	bob := scenario.remote.As(scenario.bobSpace, state.personKeyPath("bob"))
	if err := startDevelopmentServer(ctx, alice, 18080, "first", "product/alice-development-server"); err != nil {
		return err
	}
	if err := startDevelopmentServer(ctx, bob, 18081, "bob", "product/bob-development-server"); err != nil {
		return err
	}
	if err := state.waitForDevelopmentServer(ctx, "127.0.0.1", 18080, "first\n", "product/alice-development-server-lan-first"); err != nil {
		return err
	}
	if err := state.waitForDevelopmentServer(ctx, scenario.tailnetHost, 18080, "first\n", "product/alice-development-server-tailnet-first"); err != nil {
		return err
	}
	if err := state.waitForDevelopmentServer(ctx, "127.0.0.1", 18081, "bob\n", "product/bob-development-server-lan"); err != nil {
		return err
	}
	if err := state.waitForDevelopmentServer(ctx, scenario.tailnetHost, 18081, "bob\n", "product/bob-development-server-tailnet"); err != nil {
		return err
	}
	if err := alice.Capture(ctx, "product/hot-reload-write", []byte("printf 'second\\n' >\"$HOME/Projects/kept/hot-reload.txt\"\n"), "/bin/bash", "-s"); err != nil {
		return err
	}
	if err := state.waitForDevelopmentServer(ctx, "127.0.0.1", 18080, "second\n", "product/alice-hot-reload-lan"); err != nil {
		return err
	}
	return state.waitForDevelopmentServer(ctx, scenario.tailnetHost, 18080, "second\n", "product/alice-hot-reload-tailnet")
}

func startDevelopmentServer(ctx context.Context, remote Remote, port int, value, evidence string) error {
	script := fmt.Sprintf("set -eu; cd \"$HOME/Projects/kept\"; printf '%%s\\n' %s > hot-reload.txt; nohup python3 -m http.server %d </dev/null >\"$HOME/development-server.log\" 2>&1 & pid=$!; test \"$(ps -o user= -p \"$pid\" | xargs)\" = \"$(id -un)\"", strconv.Quote(value), port)
	return remote.Capture(ctx, evidence, []byte(script), "/bin/bash", "-s")
}

func (state *runnerState) waitForDevelopmentServer(ctx context.Context, host string, port int, expected, evidence string) error {
	if host == "" {
		return errors.New("development-server endpoint host is unavailable")
	}
	url := "http://" + urlHost(host) + ":" + strconv.Itoa(port) + "/hot-reload.txt"
	for attempt := 0; attempt < 20; attempt++ {
		output, err := CommandOutput(ctx, CommandSpec{Name: "curl", Args: []string{"--fail", "--silent", "--show-error", url}})
		if err == nil && string(output) == expected {
			return state.evidence.Write(evidence+".txt", output)
		}
		if err = waitBriefly(ctx); err != nil {
			return err
		}
	}
	return errors.New("project development server did not expose the expected hot-reload content")
}

func (state *runnerState) verifyMiseOwnership(ctx context.Context, scenario *scenarioState) error {
	workspaces := []struct {
		label  string
		remote Remote
	}{
		{label: "admin", remote: scenario.remote.As(scenario.adminSpace, state.paths.adminKey)},
		{label: "bob", remote: scenario.remote.As(scenario.bobSpace, state.personKeyPath("bob"))},
	}
	script := `set -euo pipefail
cd "$HOME/Projects/kept"
cat >mise.toml <<'EOF'
[tools]
node = "22.14.0"
EOF
mise trust mise.toml
mise install --include-lazy
test "$(mise exec -- node --version)" = v22.14.0
test -d "$HOME/.local/share/mise/installs/node/22.14.0"
test -d "$HOME/.cache/mise"
test ! -e /var/lib/soda/mise
test ! -e "$HOME/.config/tea/config.yml"
test ! -e "$HOME/.config/gh/hosts.yml"
printf 'workspace=%s\n' "$(id -un)"
printf 'mise_data=%s\n' "$HOME/.local/share/mise"
printf 'mise_cache=%s\n' "$HOME/.cache/mise"
`
	errorsByWorkspace := make(chan error, len(workspaces))
	for _, workspace := range workspaces {
		workspace := workspace
		go func() {
			errorsByWorkspace <- workspace.remote.Capture(ctx, "product/mise-native-"+workspace.label, []byte(script), "/bin/bash", "-s")
		}()
	}
	var result error
	for range workspaces {
		result = errors.Join(result, <-errorsByWorkspace)
	}
	if result != nil {
		return fmt.Errorf("concurrent native mise use: %w", result)
	}
	alice := scenario.remote.As(scenario.aliceSpace, state.personKeyPath("alice"))
	privacy := "set -euo pipefail\ntest ! -r /home/" + scenario.bobSpace + "/Projects/kept/mise.toml\ntest ! -r /home/" + scenario.bobSpace + "/.local/share/mise/installs/node/22.14.0/bin/node\n"
	if err := alice.Capture(ctx, "product/mise-workspace-privacy", []byte(privacy), "/bin/bash", "-s"); err != nil {
		return err
	}
	boundary := "set -euo pipefail; test ! -e /var/lib/soda/mise; test ! -e /opt/soda/toolchains; command -v tea; command -v gh\n"
	return scenario.remote.Sudo(ctx, scenario.password, boundary, "product/cli-ownership-boundaries")
}

func (state *runnerState) verifyWorkspaceRemoval(ctx context.Context, scenario *scenarioState) error {
	alice := scenario.remote.As("alice", state.personKeyPath("alice"))
	_, err := state.projectCall(ctx, alice, "remove-workspace", map[string]any{"id": "kept"}, "product/alice-remove-workspace")
	if err != nil {
		return err
	}
	check := "! getent passwd " + scenario.aliceSpace + " >/dev/null; getent passwd " + scenario.adminSpace + " >/dev/null; getent passwd " + scenario.bobSpace + " >/dev/null"
	if err = scenario.remote.Sudo(ctx, scenario.password, check, "product/own-workspace-removal"); err != nil {
		return err
	}
	contents, _ := json.Marshal(map[string]any{"id": "kept"})
	err = alice.Capture(ctx, "product/nonadmin-project-remove", append(contents, '\n'), "/usr/libexec/soda/soda-projects", "remove")
	if err == nil {
		return errors.New("non-administrator removed an entire project")
	}
	return nil
}

func (state *runnerState) verifyProjectRemoval(ctx context.Context, scenario *scenarioState) error {
	if _, err := state.createCatalogedForgejoProject(ctx, scenario.remote, scenario.password, forgejoProject{"removable", "Removal fixture", "product/removable-create"}); err != nil {
		return err
	}
	adminSetup, err := state.setupWorkspace(ctx, scenario.remote, scenario.password, "removable", "product/removable-admin-setup")
	if err != nil {
		return err
	}
	bob := scenario.remote.As("bob", state.personKeyPath("bob"))
	bobSetup, err := state.setupWorkspace(ctx, bob, scenario.password, "removable", "product/removable-bob-setup")
	if err != nil {
		return err
	}
	_, err = state.projectCall(ctx, scenario.remote, "remove", map[string]any{"id": "removable"}, "product/removable-remove")
	if err != nil {
		return err
	}
	script := "! getent passwd " + adminSetup.WorkspaceUsername + " >/dev/null; ! getent passwd " + bobSetup.WorkspaceUsername + " >/dev/null; curl --fail --silent \"" + forgejoLoopbackEndpoint + "/api/v1/repos/" + state.options.Administrator.Username + "/removable\" >/dev/null"
	return scenario.remote.Capture(ctx, "product/project-removal-preserves-forgejo", []byte(script), "/bin/bash", "-s")
}

func (state *runnerState) verifyPartialPersonDeletion(ctx context.Context, scenario *scenarioState) error {
	if err := state.addNativePerson(ctx, scenario.remote, "obsolete", scenario.password, "product/obsolete-add"); err != nil {
		return err
	}
	obsolete := scenario.remote.As("obsolete", state.personKeyPath("obsolete"))
	if _, err := state.setupWorkspace(ctx, obsolete, scenario.password, "kept", "product/obsolete-setup"); err != nil {
		return err
	}
	if _, err := state.createNativeForgejoRepository(ctx, obsolete, scenario.password, "owned", "product/owned-create"); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"username": "obsolete"})
	err := scenario.remote.Capture(ctx, "product/obsolete-delete-blocked", append(payload, '\n'), "/usr/libexec/soda/soda-projects", "delete-human")
	if err == nil {
		return errors.New("Forgejo accepted deletion of a repository owner")
	}
	if err = state.verifyBlockedDeletionEvidence(scenario); err != nil {
		return err
	}
	if err = state.deleteOwnedRepository(ctx, obsolete, scenario.password); err != nil {
		return err
	}
	if _, err = state.projectCall(ctx, scenario.remote, "delete-human", map[string]any{"username": "obsolete"}, "product/obsolete-delete-retry"); err != nil {
		return err
	}
	return scenario.remote.Sudo(ctx, scenario.password, "! getent passwd obsolete >/dev/null\n", "product/obsolete-deleted")
}

func (state *runnerState) verifyBlockedDeletionEvidence(scenario *scenarioState) error {
	path := filepath.Join(state.evidence.Root, "product/obsolete-delete-blocked.stderr")
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	message := string(contents)
	if !strings.Contains(message, "removed Soda workspaces") || !strings.Contains(message, "Forgejo account and primary Linux account obsolete remain") {
		return errors.New("partial deletion diagnostic did not state completed and remaining identities")
	}
	return nil
}

func (state *runnerState) deleteOwnedRepository(ctx context.Context, remote Remote, password []byte) error {
	config := fmt.Sprintf("user = %s\nrequest = \"DELETE\"\nurl = %s\nfail-with-body\nsilent\nshow-error\n", curlConfigQuote("obsolete:"+string(bytes.TrimSpace(password))), curlConfigQuote(forgejoLoopbackEndpoint+"/api/v1/repos/obsolete/owned"))
	_, err := remote.Exchange(ctx, "product/owned-native-delete", []byte(config), "curl", "--config", "-")
	return err
}

func scpArgs(remote Remote) []string {
	return []string{"-q", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=" + remote.KnownHosts, "-i", remote.Key, "-P", strconv.Itoa(remote.Port)}
}

func sftpArgs(remote Remote) []string {
	return append(scpArgs(remote), remote.Username+"@"+remote.Host)
}

func curlConfigQuote(value string) string {
	escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\t", "\\t").Replace(value)
	return "\"" + escaped + "\""
}
