package acceptance

import (
	"context"
	"errors"
	"fmt"
)

// Read the run here in order: host preparation, ISO lifetime, then an independent
// QCOW2 lifetime. Run always calls finish afterward, including on failure.
func (state *runnerState) execute(ctx context.Context, inputs runInputs) error {
	if err := state.verifyFallbackPublication(ctx); err != nil {
		return fmt.Errorf("previous published fallback: %w", err)
	}
	if err := state.prepareRegistry(ctx); err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	tailnetHost, installed, err := state.installAndOnboard(ctx, inputs)
	if err != nil {
		return fmt.Errorf("network ISO and first boot: %w", err)
	}
	if err = state.exerciseInstalledSystem(ctx, inputs, tailnetHost, installed); err != nil {
		return err
	}
	if err = state.checks.record("qcow2-cloud-init-local", state.exerciseReusableQCOW2(ctx, inputs)); err != nil {
		return fmt.Errorf("reusable QCOW2: %w", err)
	}
	return nil
}

// Seeded accounts and workspaces survive B→A→B before destructive product checks.
// Person fixtures carry access details, not cached role or account-existence claims.
func (state *runnerState) exerciseInstalledSystem(ctx context.Context, inputs runInputs, tailnetHost string, installed *guest) error {
	admin := inputs.Admin
	if err := verifyOrdinaryForgejoUser(ctx, admin, "iso/administrator-pam"); err != nil {
		return err
	}
	if err := captureCore(ctx, admin, "iso"); err != nil {
		return fmt.Errorf("installed product boundaries: %w", err)
	}
	project, err := seedPreservationState(ctx, admin, inputs.Keys)
	if err != nil {
		return fmt.Errorf("seed update and fallback state: %w", err)
	}
	candidate := guestImageReference(state.options.Ports.Registry, state.artifacts.Candidate)
	fallback := guestImageReference(state.options.Ports.Registry, state.artifacts.Fallback)
	if err = state.checks.record("update-and-fallback", exerciseFallback(ctx, project, installed, candidate, fallback)); err != nil {
		return fmt.Errorf("manual update and fallback: %w", err)
	}
	if err = exerciseProductScenarios(ctx, project, inputs.Keys, tailnetHost, state.checks); err != nil {
		return fmt.Errorf("product scenarios: %w", err)
	}
	if err = state.checks.record("packaged-boundaries", captureCore(ctx, admin, "final")); err != nil {
		return fmt.Errorf("final product capture: %w", err)
	}
	if installed.enrollment == nil {
		return errors.New("installed Tailnet cleanup was not registered")
	}
	return installed.shutdown(ctx)
}

func exerciseProductScenarios(ctx context.Context, project projectFixture, keys fixtureKeys, tailnetHost string, checks checkResults) error {
	// Appends Alice's later key; the workspace must retain only the original copy.
	if err := checks.record("workspace-boundaries-and-git-keys", verifyWorkspaceBoundaries(ctx, project, keys)); err != nil {
		return err
	}
	if err := checks.record("ssh-transports", verifySSHTransports(ctx, project.Alice.Remote)); err != nil {
		return err
	}
	if err := checks.record("development-server-access", verifyDevelopmentServer(ctx, project.Admin.Person, project.Alice, project.Bob, tailnetHost)); err != nil {
		return err
	}
	if err := checks.record("native-mise-ownership", verifyMiseOwnership(ctx, project)); err != nil {
		return err
	}
	// Alice is still non-administrative. Nothing below uses her removed workspace.
	if err := checks.record("workspace-removal", verifyWorkspaceRemoval(ctx, project)); err != nil {
		return err
	}
	// Promotes the primary Alice account to wheel, without changing her Forgejo role.
	if err := checks.record("cockpit-auth-and-independent-roles", verifyCockpitAndRoles(ctx, project.Admin, project.Alice.Person)); err != nil {
		return err
	}
	if err := checks.record("external-ssh-repository", verifyExternalSSHRepository(ctx, project.Admin.Person)); err != nil {
		return err
	}
	if err := checks.record("project-removal", verifyProjectRemoval(ctx, project.Admin.Person, project.Bob.Person)); err != nil {
		return err
	}
	return checks.record("human-removal-preserves-forgejo", verifyIndependentPersonDeletion(ctx, project.Admin.Person, keys))
}
