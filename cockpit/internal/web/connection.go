package web

import (
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
)

func personalizedSSHConfig(project daemonclient.Project, key daemonclient.SSHDeviceKey) string {
	return fmt.Sprintf("Host soda-%s\n    HostName soda.local\n    User %s\n    IdentityFile %s\n    IdentitiesOnly yes\n", project.Slug, project.UnixUser, sshConfigValue(key.IdentityFileHint))
}

func personalizedSSHCommand(project daemonclient.Project, key daemonclient.SSHDeviceKey) string {
	return fmt.Sprintf("ssh -i %s %s@soda.local", shellPath(key.IdentityFileHint), project.UnixUser)
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
