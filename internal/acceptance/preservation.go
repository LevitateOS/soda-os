package acceptance

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed preservation.sh
var stableManifestScript string

// Capture only the people and workspaces deliberately seeded for B→A→B.
// Linux and Git supply facts; this is not a copied account or repository database.
func captureManifest(ctx context.Context, project projectFixture, relative string) ([]byte, error) {
	workspaces := []workspaceFixture{project.Admin, project.Alice, project.Bob}
	var args []string
	for _, workspace := range workspaces {
		args = append(args, workspace.Person.Remote.Username, workspace.Remote.Username, workspace.ProjectID)
	}
	admin := project.Admin.Person
	script := "set -- " + remoteCommand(args) + "\n" + stableManifestScript
	snapshot, err := admin.Remote.SudoOutput(ctx, admin.LinuxPassword, script, relative)
	if err != nil {
		return nil, err
	}
	for _, workspace := range workspaces {
		person := workspace.Person
		user, err := forgejoAuthenticatedUser(ctx, person, relative+"-"+person.Remote.Username+"-forgejo")
		if err != nil {
			return nil, err
		}
		if user.Login != person.Remote.Username {
			return nil, fmt.Errorf("preservation authentication returned %q for %s", user.Login, person.Remote.Username)
		}
		encoded, err := json.Marshal(user)
		if err != nil {
			return nil, err
		}
		snapshot = append(snapshot, encoded...)
		snapshot = append(snapshot, '\n')
	}
	return snapshot, nil
}

// Native tools must exist before fallback; reinstalling afterward cannot prove
// that they survived. Later product checks still exercise concurrent native mise.
func seedPreservedTools(ctx context.Context, project projectFixture) error {
	for _, workspace := range []workspaceFixture{project.Admin, project.Alice, project.Bob} {
		if err := workspace.Remote.Capture(ctx, "seed/"+workspace.Person.Remote.Username+"-mise", []byte(miseCheckScript(workspace.ProjectID)), "/bin/bash", "-s"); err != nil {
			return err
		}
	}
	return nil
}

func seedCanonicalRepository(ctx context.Context, workspace workspaceFixture) error {
	script := "set -euo pipefail\ncd \"$HOME/Projects/" + workspace.ProjectID + "\"\n" + strings.Join([]string{
		"git checkout -B main",
		"git config user.name 'Soda acceptance'",
		"git config user.email acceptance@localhost",
		"printf 'committed preservation fixture\\n' >preserved.txt",
		"git add preserved.txt",
		"git commit -m 'Seed preservation fixture'",
		"git push origin HEAD:refs/heads/main",
	}, "\n") + "\n"
	return workspace.Remote.Capture(ctx, "seed/"+workspace.ProjectID+"-canonical-commit", []byte(script), "/bin/bash", "-s")
}
