package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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

type dialFunc func(context.Context, string) (sodav2.SodaServiceClient, io.Closer, error)

type app struct {
	dial    dialFunc
	getenv  func(string) string
	timeout time.Duration
}

func newApp() *app {
	return &app{
		dial: func(ctx context.Context, socket string) (sodav2.SodaServiceClient, io.Closer, error) {
			conn, err := grpcclient.Dial(ctx, socket)
			if err != nil {
				return nil, nil, err
			}
			return sodav2.NewSodaServiceClient(conn), conn, nil
		},
		getenv:  os.Getenv,
		timeout: defaultCommandTimeout,
	}
}

func (a *app) command() *cobra.Command {
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

func (a *app) healthCommand(socket *string) *cobra.Command {
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

func (a *app) peopleCommand(socket *string) *cobra.Command {
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
	people.AddCommand(a.addPersonCommand(socket), a.importPersonCommand(socket))
	return people
}

func (a *app) addPersonCommand(socket *string) *cobra.Command {
	var input personInput
	command := &cobra.Command{
		Use: "add",
		RunE: func(cmd *cobra.Command, _ []string) error {
			password := a.getenv("SODA_PERSON_PASSWORD")
			if password == "" {
				return errors.New("SODA_PERSON_PASSWORD is required")
			}
			parsedRole, err := roleValue(input.role)
			if err != nil {
				return err
			}
			return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
				response, callErr := client.CreatePerson(ctx, &sodav2.CreatePersonRequest{Username: input.username, DisplayName: input.displayName, Email: input.email, Role: parsedRole, Password: password})
				return personJSON(response.GetPerson()), callErr
			})
		},
	}
	input.bind(command)
	return command
}

func (a *app) importPersonCommand(socket *string) *cobra.Command {
	var input personInput
	command := &cobra.Command{Use: "import", RunE: func(cmd *cobra.Command, _ []string) error {
		parsedRole, err := roleValue(input.role)
		if err != nil {
			return err
		}
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, callErr := client.ImportPerson(ctx, &sodav2.ImportPersonRequest{Username: input.username, DisplayName: input.displayName, Email: input.email, Role: parsedRole})
			return personJSON(response.GetPerson()), callErr
		})
	}}
	input.bind(command)
	return command
}

type personInput struct{ username, displayName, email, role string }

func (input *personInput) bind(command *cobra.Command) {
	command.Flags().StringVar(&input.username, "username", "", "Linux username")
	command.Flags().StringVar(&input.displayName, "display-name", "", "display name")
	command.Flags().StringVar(&input.email, "email", "", "email address")
	command.Flags().StringVar(&input.role, "role", "developer", "role: admin or developer")
	_ = command.MarkFlagRequired("username")
	_ = command.MarkFlagRequired("display-name")
	_ = command.MarkFlagRequired("email")
}

func (a *app) projectsCommand(socket *string) *cobra.Command {
	projects := &cobra.Command{Use: "projects", Short: "Manage Soda projects"}
	projects.AddCommand(a.projectListCommand(socket), a.projectCreateCommand(socket), a.membersCommand(socket), a.workspacesCommand(socket), a.provisioningCommand(socket), a.deployKeyCommand(socket), a.toolchainCommand(socket))
	return projects
}

func (a *app) projectListCommand(socket *string) *cobra.Command {
	return &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, err := client.ListProjects(ctx, &sodav2.ListProjectsRequest{})
			return projectsJSON(response.GetProjects()), err
		})
	}}
}

func (a *app) projectCreateCommand(socket *string) *cobra.Command {
	var slug, name, profile, remoteURL, bootstrapPersonID string
	var memberIDs []string
	command := &cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, _ []string) error {
		parsedProfile, err := profileValue(profile)
		if err != nil {
			return err
		}
		source := &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}
		if remoteURL != "" {
			if bootstrapPersonID == "" {
				return errors.New("--bootstrap-person is required with --git")
			}
			source = &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Git{Git: &sodav2.GitProjectSource{RemoteUrl: remoteURL}}}
		}
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
			response, callErr := client.CreateProject(ctx, &sodav2.CreateProjectRequest{Slug: slug, Name: name, Profile: parsedProfile, Source: source, InitialPersonIds: memberIDs, BootstrapPersonId: bootstrapPersonID})
			return projectJSON(response.GetProject()), callErr
		})
	}}
	command.Flags().StringVar(&slug, "slug", "", "project slug")
	command.Flags().StringVar(&name, "name", "", "project display name")
	command.Flags().StringVar(&profile, "profile", "", "profile: web, python, rust, or go")
	command.Flags().StringVar(&remoteURL, "git", "", "Git remote URL")
	command.Flags().StringVar(&bootstrapPersonID, "bootstrap-person", "", "person ID used for the initial external clone")
	command.Flags().StringSliceVar(&memberIDs, "member", nil, "initial person ID (repeatable)")
	_ = command.MarkFlagRequired("slug")
	_ = command.MarkFlagRequired("name")
	_ = command.MarkFlagRequired("profile")
	_ = command.MarkFlagRequired("member")
	return command
}

func (a *app) membersCommand(socket *string) *cobra.Command {
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

func (a *app) workspacesCommand(socket *string) *cobra.Command {
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

func (a *app) provisioningCommand(socket *string) *cobra.Command {
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

func (a *app) deployKeyCommand(socket *string) *cobra.Command {
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

func (a *app) toolchainCommand(socket *string) *cobra.Command {
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

func (a *app) call(command *cobra.Command, socket string, operation func(context.Context, sodav2.SodaServiceClient) (any, error)) error {
	return a.callWithTimeout(command, socket, a.timeout, operation)
}

func (a *app) callWithTimeout(command *cobra.Command, socket string, timeout time.Duration, operation func(context.Context, sodav2.SodaServiceClient) (any, error)) error {
	ctx, cancel := context.WithTimeout(command.Context(), timeout)
	defer cancel()
	client, closer, err := a.dial(ctx, socket)
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
