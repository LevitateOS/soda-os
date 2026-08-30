package web

import (
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
)

func personalizedSSHConfig(project daemonclient.Project, user daemonclient.Person, key daemonclient.SSHDeviceKey, address string) string {
	return fmt.Sprintf("Host soda-%s\n    HostName %s\n    User %s\n    IdentityFile %s\n    IdentitiesOnly yes\n    SetEnv SODA_PROJECT=%s\n", project.Slug, address, user.Username, sshConfigValue(key.IdentityFileHint), project.Slug)
}

func personalizedSSHCommand(project daemonclient.Project, user daemonclient.Person, key daemonclient.SSHDeviceKey, address string) string {
	return fmt.Sprintf("ssh -o SetEnv=SODA_PROJECT=%s -i %s %s@%s", project.Slug, shellPath(key.IdentityFileHint), user.Username, address)
}

func sshConfigValue(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
	return `"` + escaped + `"`
}

func shellPath(value string) string {
	if value == "~" {
		return `"$HOME"`
	}
	if strings.HasPrefix(value, "~/") {
		return `"$HOME"/` + shellQuote(strings.TrimPrefix(value, "~/"))
	}
	return shellQuote(value)
}

func shellQuote(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `'"'"'`) + `'`
}

func sshKeyType(key daemonclient.SSHDeviceKey) string {
	value, _, _ := strings.Cut(key.PublicKey, " ")
	return value
}

func projectRemote(project daemonclient.Project) string {
	if source, ok := project.Source.(daemonclient.GitProjectSource); ok {
		return source.RemoteURL
	}
	return ""
}
