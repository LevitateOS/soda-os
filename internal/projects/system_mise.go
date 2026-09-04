package projects

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (platform *NativePlatform) InstallMiseTools(
	ctx context.Context,
	workspace Account,
	projectID string,
	workspaceTools, projectTools []string,
) error {
	if !projectIDPattern.MatchString(projectID) {
		return errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	if err := ValidateToolSelections(workspaceTools); err != nil {
		return err
	}
	if err := ValidateToolSelections(projectTools); err != nil {
		return err
	}
	shared, err := platform.prepareMiseProject(ctx, projectID)
	if err != nil {
		return err
	}
	if err = platform.runWorkspaceMise(ctx, workspace, workspace.Home, nil, "settings", "add", "shared_install_dirs", shared); err != nil {
		return fmt.Errorf("no tool selections completed; configure mise shared install directory: %w", err)
	}
	completedWorkspace := []string{}
	for index, tool := range workspaceTools {
		if err = platform.runWorkspaceMise(ctx, workspace, workspaceCheckout(workspace, projectID), []string{"MISE_SHARED_INSTALL_DIRS="}, "use", "--env", "local", tool); err != nil {
			return miseSelectionError("workspace", completedWorkspace, workspaceTools[index:], err)
		}
		completedWorkspace = append(completedWorkspace, tool)
	}
	completedProject := []string{}
	sharedEnvironment := []string{"MISE_SHARED_INSTALL_DIRS=" + shared}
	for index, tool := range projectTools {
		if err = platform.runWorkspaceMise(ctx, workspace, workspace.Home, sharedEnvironment, "install", "--shared", shared, tool); err != nil {
			return miseSelectionError("project", completedProject, projectTools[index:], err)
		}
		if err = platform.runWorkspaceMise(ctx, workspace, workspaceCheckout(workspace, projectID), sharedEnvironment, "use", tool); err != nil {
			return fmt.Errorf("shared tool %s was installed; completed project selections %s; project selections %s remain: %w", tool, selectionList(completedProject), selectionList(projectTools[index:]), err)
		}
		completedProject = append(completedProject, tool)
	}
	return nil
}

func (platform *NativePlatform) prepareMiseProject(ctx context.Context, projectID string) (string, error) {
	root := platform.MiseRoot
	if root == "" {
		root = "/var/lib/soda/mise"
	}
	project := filepath.Join(root, projectID)
	result, err := platform.run(ctx, "/usr/bin/install", "-d", "-m", "2770", "-o", "root", "-g", WorkspaceGroup, "--", project)
	if err != nil {
		return "", fmt.Errorf("prepare mise project storage: %w", err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("prepare mise project storage: %s", strings.TrimSpace(result.Stderr))
	}
	return filepath.Join(project, "installs"), nil
}

func (platform *NativePlatform) runWorkspaceMise(
	ctx context.Context,
	workspace Account,
	directory string,
	environment []string,
	args ...string,
) error {
	cleanEnvironment := []string{
		"HOME=" + workspace.Home,
		"USER=" + workspace.Username,
		"LOGNAME=" + workspace.Username,
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}
	cleanEnvironment = append(cleanEnvironment, environment...)
	runArgs := []string{"--user", workspace.Username, "--", "/usr/bin/env", "-i"}
	runArgs = append(runArgs, cleanEnvironment...)
	runArgs = append(runArgs, "/bin/sh", "-c", `umask 0002; exec "$@"`, "sh", "/usr/bin/mise")
	runArgs = append(runArgs, args...)
	result, err := platform.runner().Run(ctx, Command{Directory: directory, Name: "/usr/sbin/runuser", Args: runArgs})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		diagnostic := strings.TrimSpace(result.Stderr)
		if diagnostic == "" {
			diagnostic = strings.TrimSpace(result.Stdout)
		}
		return fmt.Errorf("mise exited with status %d: %s", result.ExitCode, diagnostic)
	}
	return nil
}

func (platform *NativePlatform) RemoveMiseProject(projectID string) error {
	if !projectIDPattern.MatchString(projectID) {
		return errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	root := platform.MiseRoot
	if root == "" {
		root = "/var/lib/soda/mise"
	}
	if err := os.RemoveAll(filepath.Join(root, projectID)); err != nil {
		return fmt.Errorf("remove shared mise project storage: %w", err)
	}
	return nil
}

func workspaceCheckout(workspace Account, projectID string) string {
	return filepath.Join(workspace.Home, "Projects", projectID)
}

func miseSelectionError(scope string, completed, remaining []string, cause error) error {
	return fmt.Errorf("completed %s selections %s; %s selections %s remain: %w", scope, selectionList(completed), scope, selectionList(remaining), cause)
}

func selectionList(tools []string) string {
	if len(tools) == 0 {
		return "none"
	}
	return strings.Join(tools, ", ")
}
