// Package sodactl implements the Soda OS administrative command-line client.
package sodactl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/LevitateOS/soda-os/internal/config"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultCommandTimeout    = 2 * time.Second
	defaultOSReadTimeout     = 30 * time.Second
	defaultOSStageTimeout    = 2 * time.Hour
	defaultOSActivateTimeout = 5 * time.Minute
)

type Dial func(context.Context, string) (sodav2.SodaServiceClient, io.Closer, error)

type App struct {
	Dial    Dial
	Getenv  func(string) string
	Timeout time.Duration
}

func New() *App {
	return &App{
		Dial: func(ctx context.Context, socket string) (sodav2.SodaServiceClient, io.Closer, error) {
			conn, err := grpcclient.Dial(ctx, socket)
			if err != nil {
				return nil, nil, err
			}
			return sodav2.NewSodaServiceClient(conn), conn, nil
		},
		Getenv:  os.Getenv,
		Timeout: defaultCommandTimeout,
	}
}

func (a *App) Command() *cobra.Command {
	var socket string
	root := &cobra.Command{
		Use:          "sodactl",
		Short:        "Administer Soda OS",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&socket, "socket", config.DefaultDaemonSocket, "sodad Unix socket")
	root.AddCommand(
		a.healthCommand(&socket),
		a.peopleCommand(&socket),
		a.projectsCommand(&socket),
		a.osCommand(&socket),
	)
	return root
}

func (a *App) osCommand(socket *string) *cobra.Command {
	osCommand := &cobra.Command{Use: "os", Short: "Administer the Soda OS base image"}
	update := &cobra.Command{Use: "update", Short: "Manually check, stage, and activate OS updates"}
	update.AddCommand(
		&cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, _ []string) error {
			return a.callWithTimeout(cmd, *socket, defaultOSReadTimeout, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
				response, err := client.GetOSUpdateStatus(ctx, &sodav2.GetOSUpdateStatusRequest{})
				return osUpdateStatusJSON(response.GetStatus()), err
			})
		}},
		&cobra.Command{Use: "check", RunE: func(cmd *cobra.Command, _ []string) error {
			return a.callWithTimeout(cmd, *socket, defaultOSReadTimeout, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
				response, err := client.CheckOSUpdate(ctx, &sodav2.CheckOSUpdateRequest{})
				return osReleaseJSON(response.GetRelease()), err
			})
		}},
		&cobra.Command{Use: "stage", RunE: func(cmd *cobra.Command, _ []string) error {
			return a.callWithTimeout(cmd, *socket, defaultOSStageTimeout, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
				checked, err := client.CheckOSUpdate(ctx, &sodav2.CheckOSUpdateRequest{})
				if err != nil {
					return nil, err
				}
				release := checked.GetRelease()
				if release == nil || !release.GetAvailable() {
					return nil, status.Error(codes.FailedPrecondition, "no newer signed Soda OS release is available")
				}
				response, err := client.StageOSUpdate(ctx, &sodav2.StageOSUpdateRequest{ImageReference: release.GetImageReference()})
				return osUpdateStatusJSON(response.GetStatus()), err
			})
		}},
	)
	var confirmReboot bool
	activate := &cobra.Command{Use: "activate", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.callWithTimeout(cmd, *socket, defaultOSActivateTimeout, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.ActivateOSUpdate(ctx, &sodav2.ActivateOSUpdateRequest{ConfirmReboot: confirmReboot})
			return map[string]any{"reboot_requested": response.GetRebootRequested()}, err
		})
	}}
	activate.Flags().BoolVar(&confirmReboot, "confirm-reboot", false, "confirm immediate maintenance reboot into the staged image")
	_ = activate.MarkFlagRequired("confirm-reboot")
	update.AddCommand(activate)
	osCommand.AddCommand(update)
	return osCommand
}

func (a *App) healthCommand(socket *string) *cobra.Command {
	return &cobra.Command{
		Use: "health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
				response, err := client.Health(ctx, &sodav2.HealthRequest{})
				return healthJSON(response), err
			})
		},
	}
}

func (a *App) peopleCommand(socket *string) *cobra.Command {
	people := &cobra.Command{Use: "people", Short: "Manage Soda people"}
	people.AddCommand(&cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
				response, err := client.ListPeople(ctx, &sodav2.ListPeopleRequest{})
				return peopleJSON(response.GetPeople()), err
			})
		},
	})
	people.AddCommand(a.personCommand(socket, false), a.personCommand(socket, true))
	return people
}

func (a *App) personCommand(socket *string, imported bool) *cobra.Command {
	var username, displayName, email, role string
	use := "add"
	if imported {
		use = "import"
	}
	command := &cobra.Command{
		Use: use,
		RunE: func(cmd *cobra.Command, _ []string) error {
			password := ""
			if !imported {
				password = a.Getenv("SODA_PERSON_PASSWORD")
				if password == "" {
					return errors.New("SODA_PERSON_PASSWORD is required")
				}
			}
			parsedRole, err := roleValue(role)
			if err != nil {
				return err
			}
			return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
				if imported {
					response, callErr := client.ImportPerson(ctx, &sodav2.ImportPersonRequest{Username: username, DisplayName: displayName, Email: email, Role: parsedRole})
					return personJSON(response.GetPerson()), callErr
				}
				response, callErr := client.CreatePerson(ctx, &sodav2.CreatePersonRequest{Username: username, DisplayName: displayName, Email: email, Role: parsedRole, Password: password})
				return personJSON(response.GetPerson()), callErr
			})
		},
	}
	command.Flags().StringVar(&username, "username", "", "Linux username")
	command.Flags().StringVar(&displayName, "display-name", "", "display name")
	command.Flags().StringVar(&email, "email", "", "email address")
	command.Flags().StringVar(&role, "role", "developer", "role: admin or developer")
	_ = command.MarkFlagRequired("username")
	_ = command.MarkFlagRequired("display-name")
	_ = command.MarkFlagRequired("email")
	return command
}

func (a *App) projectsCommand(socket *string) *cobra.Command {
	projects := &cobra.Command{Use: "projects", Short: "Manage Soda projects"}
	projects.AddCommand(a.projectListCommand(socket), a.projectCreateCommand(socket), a.membersCommand(socket), a.workspacesCommand(socket), a.provisioningCommand(socket), a.deployKeyCommand(socket), a.toolchainCommand(socket))
	return projects
}

func (a *App) projectListCommand(socket *string) *cobra.Command {
	return &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.ListProjects(ctx, &sodav2.ListProjectsRequest{})
			return projectsJSON(response.GetProjects()), err
		})
	}}
}

func (a *App) projectCreateCommand(socket *string) *cobra.Command {
	var slug, name, profile, remoteURL string
	var memberIDs []string
	command := &cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, _ []string) error {
		parsedProfile, err := profileValue(profile)
		if err != nil {
			return err
		}
		source := &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}
		if remoteURL != "" {
			source = &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Git{Git: &sodav2.GitProjectSource{RemoteUrl: remoteURL}}}
		}
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, callErr := client.CreateProject(ctx, &sodav2.CreateProjectRequest{Slug: slug, Name: name, Profile: parsedProfile, Source: source, InitialPersonIds: memberIDs})
			return projectJSON(response.GetProject()), callErr
		})
	}}
	command.Flags().StringVar(&slug, "slug", "", "project slug")
	command.Flags().StringVar(&name, "name", "", "project display name")
	command.Flags().StringVar(&profile, "profile", "", "profile: web, python, rust, or go")
	command.Flags().StringVar(&remoteURL, "git", "", "Git remote URL")
	command.Flags().StringSliceVar(&memberIDs, "member", nil, "initial person ID (repeatable)")
	_ = command.MarkFlagRequired("slug")
	_ = command.MarkFlagRequired("name")
	_ = command.MarkFlagRequired("profile")
	_ = command.MarkFlagRequired("member")
	return command
}

func (a *App) membersCommand(socket *string) *cobra.Command {
	var projectID, personID string
	group := &cobra.Command{Use: "members", Short: "Manage project members"}
	add := &cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.AddCollaborator(ctx, &sodav2.AddCollaboratorRequest{ProjectId: projectID, PersonId: personID})
			return worktreeJSON(response.GetWorktree()), err
		})
	}}
	add.Flags().StringVar(&projectID, "project", "", "project ID")
	add.Flags().StringVar(&personID, "person", "", "person ID")
	_ = add.MarkFlagRequired("project")
	_ = add.MarkFlagRequired("person")
	var listProjectID string
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.ListCollaborators(ctx, &sodav2.ListCollaboratorsRequest{ProjectId: listProjectID})
			return collaboratorsJSON(response.GetCollaborators()), err
		})
	}}
	list.Flags().StringVar(&listProjectID, "project", "", "project ID")
	_ = list.MarkFlagRequired("project")
	group.AddCommand(add, list)
	return group
}

func (a *App) workspacesCommand(socket *string) *cobra.Command {
	group := &cobra.Command{Use: "workspaces", Short: "Inspect personal workspaces"}
	var listProjectID string
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.ListWorktrees(ctx, &sodav2.ListWorktreesRequest{ProjectId: listProjectID})
			return worktreesJSON(response.GetWorktrees()), err
		})
	}}
	list.Flags().StringVar(&listProjectID, "project", "", "project ID")
	_ = list.MarkFlagRequired("project")
	group.AddCommand(list)
	return group
}

func (a *App) provisioningCommand(socket *string) *cobra.Command {
	group := &cobra.Command{Use: "provisioning", Short: "Inspect and retry provisioning"}
	var listProjectID string
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.ListProvisioningJobs(ctx, &sodav2.ListProvisioningJobsRequest{ProjectId: listProjectID})
			return jobsJSON(response.GetJobs()), err
		})
	}}
	list.Flags().StringVar(&listProjectID, "project", "", "project ID")
	_ = list.MarkFlagRequired("project")
	var retryProjectID string
	retry := &cobra.Command{Use: "retry", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.StartProvisioning(ctx, &sodav2.StartProvisioningRequest{ProjectId: retryProjectID})
			return jobJSON(response.GetJob()), err
		})
	}}
	retry.Flags().StringVar(&retryProjectID, "project", "", "project ID")
	_ = retry.MarkFlagRequired("project")
	group.AddCommand(list, retry)
	return group
}

func (a *App) deployKeyCommand(socket *string) *cobra.Command {
	var projectID string
	command := &cobra.Command{Use: "deploy-key", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.GetDeployKey(ctx, &sodav2.GetDeployKeyRequest{ProjectId: projectID})
			return deployKeyJSON(response.GetDeployKey()), err
		})
	}}
	command.Flags().StringVar(&projectID, "project", "", "project ID")
	_ = command.MarkFlagRequired("project")
	return command
}

func (a *App) toolchainCommand(socket *string) *cobra.Command {
	var projectID string
	command := &cobra.Command{Use: "toolchain", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.GetProjectToolchain(ctx, &sodav2.GetProjectToolchainRequest{ProjectId: projectID})
			return toolchainJSON(response.GetInstallation()), err
		})
	}}
	command.Flags().StringVar(&projectID, "project", "", "project ID")
	_ = command.MarkFlagRequired("project")
	return command
}

func (a *App) call(command *cobra.Command, socket string, operation func(context.Context, sodav2.SodaServiceClient) (any, error)) error {
	return a.callWithTimeout(command, socket, a.Timeout, operation)
}

func (a *App) callWithTimeout(command *cobra.Command, socket string, timeout time.Duration, operation func(context.Context, sodav2.SodaServiceClient) (any, error)) error {
	ctx, cancel := context.WithTimeout(command.Context(), timeout)
	defer cancel()
	client, closer, err := a.Dial(ctx, socket)
	if err != nil {
		return errors.New("sodad unavailable: Soda service is unavailable")
	}
	defer closer.Close()
	response, err := operation(ctx, client)
	if err != nil {
		return canonicalError(err)
	}
	encoded, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	_, err = fmt.Fprintln(command.Root().OutOrStdout(), string(encoded))
	return err
}

func roleValue(input string) (sodav2.Role, error) {
	switch input {
	case "admin":
		return sodav2.Role_ROLE_ADMIN, nil
	case "developer":
		return sodav2.Role_ROLE_DEVELOPER, nil
	default:
		return sodav2.Role_ROLE_UNSPECIFIED, fmt.Errorf("invalid role %q; expected admin or developer", input)
	}
}

func profileValue(input string) (sodav2.ToolchainProfile, error) {
	switch input {
	case "web":
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_WEB, nil
	case "python":
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_PYTHON, nil
	case "rust":
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_RUST, nil
	case "go":
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO, nil
	default:
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_UNSPECIFIED, fmt.Errorf("invalid profile %q; expected web, python, rust, or go", input)
	}
}

func canonicalError(err error) error {
	grpcStatus, ok := status.FromError(err)
	if !ok {
		return err
	}
	prefix := "sodad error"
	switch grpcStatus.Code() {
	case codes.InvalidArgument:
		prefix = "invalid input"
	case codes.NotFound:
		prefix = "not found"
	case codes.AlreadyExists:
		prefix = "conflict"
	case codes.PermissionDenied, codes.Unauthenticated:
		prefix = "permission denied"
	case codes.FailedPrecondition:
		prefix = "operation rejected"
	case codes.Unavailable:
		return errors.New("sodad unavailable: Soda service is unavailable")
	case codes.DeadlineExceeded:
		return errors.New("sodad timed out: Soda service did not respond in time")
	case codes.Internal, codes.Unknown:
		return errors.New("sodad error: internal service error")
	}
	return fmt.Errorf("%s: %s", prefix, grpcStatus.Message())
}

func healthJSON(health *sodav2.HealthResponse) any {
	if health == nil {
		return nil
	}
	return map[string]any{"status": health.Status, "service": health.Service, "version": health.Version}
}

func personJSON(person *sodav2.Person) any {
	if person == nil {
		return nil
	}
	return map[string]any{
		"id": person.Id, "username": person.Username, "display_name": person.DisplayName,
		"email": person.Email, "role": roleName(person.Role),
	}
}

func peopleJSON(people []*sodav2.Person) []any {
	result := make([]any, len(people))
	for index, person := range people {
		result[index] = personJSON(person)
	}
	return result
}

func projectJSON(project *sodav2.Project) any {
	if project == nil {
		return nil
	}
	return map[string]any{
		"id": project.Id, "slug": project.Slug, "name": project.Name, "unix_user": project.UnixUser,
		"profile": profileName(project.Profile), "source": projectSourceJSON(project.Source),
	}
}

func projectsJSON(projects []*sodav2.Project) []any {
	result := make([]any, len(projects))
	for index, project := range projects {
		result[index] = projectJSON(project)
	}
	return result
}

func projectSourceJSON(source *sodav2.ProjectSource) any {
	if source == nil || source.GetEmpty() != nil {
		return map[string]any{"kind": "empty"}
	}
	if git := source.GetGit(); git != nil {
		return map[string]any{"kind": "git", "remote_url": git.RemoteUrl}
	}
	return nil
}

func membershipJSON(membership *sodav2.Membership) any {
	if membership == nil {
		return nil
	}
	return map[string]any{"project_id": membership.ProjectId, "person_id": membership.PersonId}
}

func collaboratorJSON(collaborator *sodav2.Collaborator) any {
	if collaborator == nil {
		return nil
	}
	return map[string]any{
		"person": personJSON(collaborator.Person), "membership": membershipJSON(collaborator.Membership),
		"workspaces": worktreesJSON(collaborator.Worktrees),
	}
}

func collaboratorsJSON(collaborators []*sodav2.Collaborator) []any {
	result := make([]any, len(collaborators))
	for index, collaborator := range collaborators {
		result[index] = collaboratorJSON(collaborator)
	}
	return result
}

func worktreeJSON(worktree *sodav2.Worktree) any {
	if worktree == nil {
		return nil
	}
	return map[string]any{
		"id": worktree.Id, "project_id": worktree.ProjectId, "person_id": worktree.PersonId,
		"name": worktree.Name, "branch": worktree.Branch, "path": worktree.Path,
	}
}

func worktreesJSON(worktrees []*sodav2.Worktree) []any {
	result := make([]any, len(worktrees))
	for index, worktree := range worktrees {
		result[index] = worktreeJSON(worktree)
	}
	return result
}

func jobJSON(job *sodav2.ProvisioningJob) any {
	if job == nil {
		return nil
	}
	var jobError any
	if job.Error != nil {
		jobError = job.GetError()
	}
	return map[string]any{"id": job.Id, "project_id": job.ProjectId, "state": jobStateName(job.State), "error": jobError}
}

func jobsJSON(jobs []*sodav2.ProvisioningJob) []any {
	result := make([]any, len(jobs))
	for index, job := range jobs {
		result[index] = jobJSON(job)
	}
	return result
}

func deployKeyJSON(key *sodav2.DeployKey) any {
	if key == nil {
		return nil
	}
	return map[string]any{"project_id": key.ProjectId, "public_key": key.PublicKey}
}

func toolchainJSON(toolchain *sodav2.ToolchainInstallation) any {
	if toolchain == nil {
		return nil
	}
	return map[string]any{
		"id": toolchain.Id, "profile": profileName(toolchain.Profile), "version": toolchain.Version,
		"path": toolchain.Path, "checksum": toolchain.Checksum, "state": jobStateName(toolchain.State),
	}
}

func osDeploymentJSON(deployment *sodav2.OSDeployment) any {
	if deployment == nil {
		return nil
	}
	return map[string]any{
		"image_reference": deployment.ImageReference, "version": deployment.Version, "digest": deployment.Digest,
		"architecture": deployment.Architecture, "signature": deployment.Signature,
		"incompatible": deployment.Incompatible, "download_only": deployment.DownloadOnly,
	}
}

func osUpdateStatusJSON(value *sodav2.OSUpdateStatus) any {
	if value == nil {
		return nil
	}
	return map[string]any{"booted": osDeploymentJSON(value.Booted), "staged": osDeploymentJSON(value.Staged), "read_only": value.ReadOnly}
}

func osReleaseJSON(value *sodav2.OSRelease) any {
	if value == nil {
		return nil
	}
	return map[string]any{
		"image_reference": value.ImageReference, "version": value.Version,
		"source_revision": value.SourceRevision, "fedora_base_reference": value.FedoraBaseReference,
		"digest": value.Digest, "state_schema": value.StateSchema, "available": value.Available,
	}
}

func roleName(role sodav2.Role) string {
	return strings.ToLower(strings.TrimPrefix(role.String(), "ROLE_"))
}

func profileName(profile sodav2.ToolchainProfile) string {
	return strings.ToLower(strings.TrimPrefix(profile.String(), "TOOLCHAIN_PROFILE_"))
}

func jobStateName(state sodav2.JobState) string {
	return strings.ToLower(strings.TrimPrefix(state.String(), "JOB_STATE_"))
}
