package acceptance

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func captureCore(ctx context.Context, admin personFixture, prefix string) error {
	if err := admin.Remote.Capture(ctx, prefix+"/core", []byte(coreGuestChecks), "/bin/bash", "-s"); err != nil {
		return err
	}
	if err := admin.Remote.Sudo(ctx, admin.LinuxPassword, tailscaleAccessCheck, prefix+"/tailscale-access"); err != nil {
		return err
	}
	return admin.Remote.Sudo(ctx, admin.LinuxPassword, stableManifestScript, prefix+"/system-manifest")
}

func runQCOW2Checks(ctx context.Context, admin personFixture, originalVirtualSize int64) error {
	remote, password := admin.Remote, admin.LinuxPassword
	if err := remote.Capture(ctx, "qcow2/core", []byte(coreGuestChecks), "/bin/bash", "-s"); err != nil {
		return err
	}
	if err := remote.Sudo(ctx, password, qcow2GuestChecks, "qcow2/cloud-init"); err != nil {
		return err
	}
	growthCheck := fmt.Sprintf("set -euo pipefail\nroot_bytes=$(df -B1 --output=size / | tail -n 1 | tr -d ' ')\ntest \"$root_bytes\" -gt %d\nprintf 'original_virtual_size=%%s\\nusable_root_size=%%s\\n' %d \"$root_bytes\"\n", originalVirtualSize, originalVirtualSize)
	if err := remote.Sudo(ctx, password, growthCheck, "qcow2/volume-growth"); err != nil {
		return fmt.Errorf("verify reusable QCOW2 guest volume growth: %w", err)
	}
	if err := verifyLocalProjectsWithoutTailscale(ctx, admin); err != nil {
		return err
	}
	return remote.Sudo(ctx, password, nativeServiceChecks, "qcow2/native-service-state")
}

func verifyLocalProjectsWithoutTailscale(ctx context.Context, admin personFixture) error {
	projects, err := catalogProjects(ctx, admin.Remote, "qcow2/projects-list-without-tailscale")
	if err != nil {
		return err
	}
	if len(projects) != 0 {
		return errors.New("reusable QCOW2 project catalog is not empty before local-only setup")
	}
	if _, err = createCatalogedForgejoProject(ctx, admin, forgejoProject{ID: "local-only", Name: "Local-only project", Evidence: "qcow2/local-project-create"}); err != nil {
		return err
	}
	if _, err = setupWorkspace(ctx, admin, "local-only", "qcow2/local-project-setup"); err != nil {
		return err
	}
	return admin.Remote.Sudo(ctx, admin.LinuxPassword, `set -euo pipefail
tailscale status --json | jq -e '.BackendState != "Running"' >/dev/null
`, "qcow2/projects-setup-without-tailscale")
}

func seedPreservationState(ctx context.Context, admin personFixture, keys fixtureKeys) (projectFixture, error) {
	if _, err := createCatalogedForgejoProject(ctx, admin, forgejoProject{"kept", "Kept project", "seed/kept-create"}); err != nil {
		return projectFixture{}, err
	}
	adminSpace, err := setupWorkspace(ctx, admin, "kept", "seed/admin-setup")
	if err != nil {
		return projectFixture{}, err
	}
	alice, err := addNativePerson(ctx, admin, "alice", keys, "seed/alice-add")
	if err != nil {
		return projectFixture{}, err
	}
	aliceSpace, err := setupWorkspace(ctx, alice, "kept", "seed/alice-setup")
	if err != nil {
		return projectFixture{}, err
	}
	bob, err := addNativePerson(ctx, admin, "bob", keys, "seed/bob-add")
	if err != nil {
		return projectFixture{}, err
	}
	bobSpace, err := setupWorkspace(ctx, bob, "kept", "seed/bob-setup")
	if err != nil {
		return projectFixture{}, err
	}
	if err = editCatalogMetadata(ctx, alice.Remote, bob.Remote); err != nil {
		return projectFixture{}, err
	}
	project := projectFixture{Admin: adminSpace, Alice: aliceSpace, Bob: bobSpace}
	if err = seedWorkspaceFiles(ctx, project); err != nil {
		return projectFixture{}, err
	}
	return project, nil
}

func seedWorkspaceFiles(ctx context.Context, project projectFixture) error {
	for _, item := range []struct {
		label     string
		workspace workspaceFixture
	}{{"admin", project.Admin}, {"alice", project.Alice}, {"bob", project.Bob}} {
		script := "set -eu; printf '%s-private\\n' " + item.label + " >\"$HOME/Projects/" + item.workspace.ProjectID + "/" + item.label + "-private.txt\"; printf 'preserved\\n' >\"$HOME/soda-acceptance-state.txt\""
		if err := item.workspace.Remote.Capture(ctx, "seed/"+item.label+"-workspace-state", []byte(script), "/bin/bash", "-s"); err != nil {
			return err
		}
	}
	admin := project.Admin.Person
	return admin.Remote.Sudo(ctx, admin.LinuxPassword, workspaceCheckScript(admin.Remote.Username, project.Admin.ProjectID, project.Admin.Remote.Username), "seed/workspace-boundary")
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
