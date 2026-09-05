package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func verifyWorkspaceBoundaries(ctx context.Context, project projectFixture, keys fixtureKeys) error {
	admin := project.Admin.Person
	for _, item := range []struct {
		label     string
		workspace workspaceFixture
	}{{"admin", project.Admin}, {"alice", project.Alice}, {"bob", project.Bob}} {
		script := workspaceCheckScript(item.workspace.Person.Remote.Username, item.workspace.ProjectID, item.workspace.Remote.Username)
		if err := admin.Remote.Sudo(ctx, admin.LinuxPassword, script, "product/"+item.label+"-workspace-boundary"); err != nil {
			return err
		}
	}
	if err := verifyIndependentWorkspaceUIDs(ctx, project); err != nil {
		return err
	}
	if err := verifyWorkspaceGitKeys(ctx, project); err != nil {
		return err
	}
	if err := verifyWorkspaceForgejoAbsence(ctx, project); err != nil {
		return err
	}
	return verifyOneTimeAuthorizedKeys(ctx, project.Alice, keys)
}

func verifyIndependentWorkspaceUIDs(ctx context.Context, project projectFixture) error {
	script := fmt.Sprintf("test \"$(printf '%%s\\n' $(id -u %s) $(id -u %s) $(id -u %s) | sort -u | wc -l)\" -eq 3", project.Admin.Remote.Username, project.Alice.Remote.Username, project.Bob.Remote.Username)
	return project.Admin.Person.Remote.Sudo(ctx, project.Admin.Person.LinuxPassword, script, "product/workspace-uids")
}

func verifyWorkspaceForgejoAbsence(ctx context.Context, project projectFixture) error {
	script := "set -eu; url=" + forgejoLoopbackEndpoint + "; for user in " + project.Admin.Remote.Username + " " + project.Alice.Remote.Username + " " + project.Bob.Remote.Username + "; do test \"$(curl --silent --output /dev/null --write-out '%{http_code}' \"$url/api/v1/users/$user\")\" = 404; done"
	return project.Admin.Person.Remote.Capture(ctx, "product/workspace-forgejo-absence", []byte(script), "/bin/bash", "-s")
}

func verifySSHTransports(ctx context.Context, workspace Remote) error {
	if err := workspace.Capture(ctx, "product/direct-command", nil, "id"); err != nil {
		return err
	}
	source := filepath.Join(workspace.Evidence.Root, "product", "scp-input.txt")
	if err := workspace.Evidence.Write("product/scp-input.txt", []byte("scp-product-evidence\n")); err != nil {
		return err
	}
	args := append(scpArgs(workspace), source, workspace.Username+"@"+workspace.Host+":scp-input.txt")
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
	return workspace.Evidence.Write("product/sftp.txt", output)
}

func verifyCockpitAndRoles(ctx context.Context, adminWorkspace workspaceFixture, alice personFixture) error {
	admin := adminWorkspace.Person
	status, err := cockpitLoginStatus(ctx, admin.Remote, admin.Remote.Username, admin.LinuxPassword)
	if err != nil || status != "200" {
		return fmt.Errorf("primary Cockpit authentication returned %s: %w", status, err)
	}
	status, err = cockpitLoginStatus(ctx, admin.Remote, adminWorkspace.Remote.Username, []byte("locked-workspace-password"))
	if err != nil || status != "401" {
		return fmt.Errorf("workspace Cockpit authentication returned %s: %w", status, err)
	}
	if err = admin.Remote.Sudo(ctx, admin.LinuxPassword, "/usr/sbin/usermod --append --groups wheel -- "+alice.Remote.Username+"\n", "product/alice-wheel-promotion"); err != nil {
		return err
	}
	response, err := forgejoAuthenticatedUser(ctx, alice.Remote, alice.Remote.Username, alice.ForgejoPassword)
	if err != nil {
		return err
	}
	if response.Login != alice.Remote.Username || response.IsAdmin {
		return errors.New("Linux wheel promotion changed native Forgejo administration")
	}
	return admin.Remote.Evidence.Write("product/cockpit-status.txt", []byte("primary=200\nworkspace=401\n"))
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

func requestForgejoUser(ctx context.Context, remote Remote, username string, password []byte) (CommandResult, error) {
	config := fmt.Sprintf("user = %s\nsilent\nshow-error\nfail-with-body\nurl = %s\n", curlConfigQuote(username+":"+string(bytes.TrimSpace(password))), curlConfigQuote(forgejoLoopbackEndpoint+"/api/v1/user"))
	return remote.Exchange(ctx, "product/"+username+"-forgejo-user", []byte(config), "curl", "--config", "-")
}

func forgejoAuthenticatedUser(ctx context.Context, remote Remote, username string, password []byte) (forgejoUser, error) {
	result, err := requestForgejoUser(ctx, remote, username, password)
	if err = errors.Join(result.Err, err); err != nil {
		return forgejoUser{}, err
	}
	var user forgejoUser
	err = json.Unmarshal(result.Stdout, &user)
	return user, err
}

func verifyDevelopmentServer(ctx context.Context, alice, bob workspaceFixture, tailnetHost string) error {
	if tailnetHost == "" {
		return errors.New("development-server endpoint host is unavailable")
	}
	if err := startDevelopmentServer(ctx, alice, 18080, "first", "product/alice-development-server"); err != nil {
		return err
	}
	if err := startDevelopmentServer(ctx, bob, 18081, "bob", "product/bob-development-server"); err != nil {
		return err
	}
	evidence := alice.Remote.Evidence
	if err := waitForDevelopmentServer(ctx, urlHost(alice.Remote.Host)+":18080", "first\n", "product/alice-development-server-local-forwarded-first", evidence); err != nil {
		return err
	}
	if err := waitForDevelopmentServer(ctx, urlHost(tailnetHost)+":18080", "first\n", "product/alice-development-server-tailnet-first", evidence); err != nil {
		return err
	}
	if err := waitForDevelopmentServer(ctx, urlHost(bob.Remote.Host)+":18081", "bob\n", "product/bob-development-server-local-forwarded", evidence); err != nil {
		return err
	}
	if err := waitForDevelopmentServer(ctx, urlHost(tailnetHost)+":18081", "bob\n", "product/bob-development-server-tailnet", evidence); err != nil {
		return err
	}
	if err := alice.Remote.Capture(ctx, "product/hot-reload-write", []byte("printf 'second\\n' >\"$HOME/Projects/"+alice.ProjectID+"/hot-reload.txt\"\n"), "/bin/bash", "-s"); err != nil {
		return err
	}
	if err := waitForDevelopmentServer(ctx, urlHost(alice.Remote.Host)+":18080", "second\n", "product/alice-hot-reload-local-forwarded", evidence); err != nil {
		return err
	}
	return waitForDevelopmentServer(ctx, urlHost(tailnetHost)+":18080", "second\n", "product/alice-hot-reload-tailnet", evidence)
}

func startDevelopmentServer(ctx context.Context, workspace workspaceFixture, port int, value, evidence string) error {
	script := fmt.Sprintf("set -eu; cd \"$HOME/Projects/%s\"; printf '%%s\\n' %s > hot-reload.txt; nohup python3 -m http.server %d </dev/null >\"$HOME/development-server.log\" 2>&1 & pid=$!; test \"$(ps -o user= -p \"$pid\" | xargs)\" = \"$(id -un)\"", workspace.ProjectID, strconv.Quote(value), port)
	return workspace.Remote.Capture(ctx, evidence, []byte(script), "/bin/bash", "-s")
}

func waitForDevelopmentServer(ctx context.Context, address, expected, label string, evidence Evidence) error {
	url := "http://" + address + "/hot-reload.txt"
	for attempt := 0; attempt < 20; attempt++ {
		output, err := CommandOutput(ctx, CommandSpec{Name: "curl", Args: []string{"--fail", "--silent", "--show-error", url}})
		if err == nil && string(output) == expected {
			return evidence.Write(label+".txt", output)
		}
		if err = waitBriefly(ctx); err != nil {
			return err
		}
	}
	return errors.New("project development server did not expose the expected hot-reload content")
}

func verifyMiseOwnership(ctx context.Context, project projectFixture) error {
	workspaces := []struct {
		label     string
		workspace workspaceFixture
	}{{"admin", project.Admin}, {"bob", project.Bob}}
	errorsByWorkspace := make(chan error, len(workspaces))
	for _, item := range workspaces {
		go func() {
			errorsByWorkspace <- item.workspace.Remote.Capture(ctx, "product/mise-native-"+item.label, []byte(miseCheckScript(item.workspace.ProjectID)), "/bin/bash", "-s")
		}()
	}
	var result error
	for range workspaces {
		result = errors.Join(result, <-errorsByWorkspace)
	}
	if result != nil {
		return fmt.Errorf("concurrent native mise use: %w", result)
	}
	privacy := "set -euo pipefail\ntest ! -r /home/" + project.Bob.Remote.Username + "/Projects/" + project.Bob.ProjectID + "/mise.toml\ntest ! -r /home/" + project.Bob.Remote.Username + "/.local/share/mise/installs/node/22.14.0/bin/node\n"
	if err := project.Alice.Remote.Capture(ctx, "product/mise-workspace-privacy", []byte(privacy), "/bin/bash", "-s"); err != nil {
		return err
	}
	boundary := "set -euo pipefail; test ! -e /var/lib/soda/mise; test ! -e /opt/soda/toolchains; command -v tea; command -v gh\n"
	return project.Admin.Person.Remote.Sudo(ctx, project.Admin.Person.LinuxPassword, boundary, "product/cli-ownership-boundaries")
}

func miseCheckScript(project string) string {
	return `set -euo pipefail
cd "$HOME/Projects/` + project + `"
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
}

func verifyWorkspaceRemoval(ctx context.Context, project projectFixture) error {
	alice := project.Alice.Person.Remote
	_, err := projectRemoval(ctx, alice, "remove-workspace", project.Alice.ProjectID, "product/alice-remove-workspace")
	if err != nil {
		return err
	}
	check := "! getent passwd " + project.Alice.Remote.Username + " >/dev/null; getent passwd " + project.Admin.Remote.Username + " >/dev/null; getent passwd " + project.Bob.Remote.Username + " >/dev/null"
	if err = project.Admin.Person.Remote.Sudo(ctx, project.Admin.Person.LinuxPassword, check, "product/own-workspace-removal"); err != nil {
		return err
	}
	result, err := invokeProject(ctx, alice, "remove", map[string]any{"id": project.Alice.ProjectID, "expected": "reviewed"}, "product/nonadmin-project-remove")
	if err != nil {
		return err
	}
	if result.Err == nil {
		return errors.New("non-administrator removed an entire project")
	}
	return nil
}

func verifyProjectRemoval(ctx context.Context, admin, bob personFixture) error {
	if _, err := createCatalogedForgejoProject(ctx, admin, forgejoProject{"removable", "Removal fixture", "product/removable-create"}); err != nil {
		return err
	}
	adminSetup, err := setupWorkspace(ctx, admin, "removable", "product/removable-admin-setup")
	if err != nil {
		return err
	}
	bobSetup, err := setupWorkspace(ctx, bob, "removable", "product/removable-bob-setup")
	if err != nil {
		return err
	}
	if _, err = projectRemoval(ctx, admin.Remote, "remove", "removable", "product/removable-remove"); err != nil {
		return err
	}
	script := "! getent passwd " + adminSetup.Remote.Username + " >/dev/null; ! getent passwd " + bobSetup.Remote.Username + " >/dev/null; curl --fail --silent \"" + forgejoLoopbackEndpoint + "/api/v1/repos/" + admin.Remote.Username + "/removable\" >/dev/null"
	return admin.Remote.Capture(ctx, "product/project-removal-preserves-forgejo", []byte(script), "/bin/bash", "-s")
}

func verifyIndependentPersonDeletion(ctx context.Context, admin personFixture, keys fixtureKeys) error {
	obsolete, err := addNativePerson(ctx, admin, "obsolete", keys, "product/obsolete-add")
	if err != nil {
		return err
	}
	if _, err = setupWorkspace(ctx, obsolete, "kept", "product/obsolete-setup"); err != nil {
		return err
	}
	if _, err = createNativeForgejoRepository(ctx, obsolete, "owned", "product/owned-create"); err != nil {
		return err
	}
	if _, err = projectRemoval(ctx, admin.Remote, "delete-human", "obsolete", "product/obsolete-delete"); err != nil {
		return err
	}
	script := "! getent passwd obsolete >/dev/null\n" +
		"curl --fail --silent " + forgejoLoopbackEndpoint + "/api/v1/users/obsolete >/dev/null\n" +
		"curl --fail --silent " + forgejoLoopbackEndpoint + "/api/v1/repos/obsolete/owned >/dev/null\n"
	return admin.Remote.Sudo(ctx, admin.LinuxPassword, script, "product/linux-deletion-preserves-forgejo")
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
