// Package sodactl implements the Soda OS administrative command-line client.
package sodactl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LevitateOS/soda-os/internal/config"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Dial func(context.Context, string) (sodav2.SodaServiceClient, io.Closer, error)

type App struct {
	Dial     Dial
	Getenv   func(string) string
	ReadFile func(string) ([]byte, error)
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
		Getenv:   os.Getenv,
		ReadFile: os.ReadFile,
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
	)
	return root
}

func (a *App) healthCommand(socket *string) *cobra.Command {
	return &cobra.Command{
		Use: "health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
				return client.Health(ctx, &sodav2.HealthRequest{})
			})
		},
	}
}

func (a *App) peopleCommand(socket *string) *cobra.Command {
	people := &cobra.Command{Use: "people", Short: "Manage Soda people"}
	people.AddCommand(&cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
				return client.ListPeople(ctx, &sodav2.ListPeopleRequest{})
			})
		},
	})
	people.AddCommand(a.personCommand(socket, false), a.personCommand(socket, true))
	return people
}

func (a *App) personCommand(socket *string, imported bool) *cobra.Command {
	var username, displayName, email, role, keyPath string
	use := "add"
	if imported {
		use = "import"
	}
	command := &cobra.Command{
		Use: use,
		RunE: func(cmd *cobra.Command, _ []string) error {
			key, err := a.readKey(keyPath)
			if err != nil {
				return err
			}
			parsedRole, err := roleValue(role)
			if err != nil {
				return err
			}
			return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
				if imported {
					return client.ImportPerson(ctx, &sodav2.ImportPersonRequest{Username: username, DisplayName: displayName, Email: email, Role: parsedRole, SshPublicKey: key})
				}
				password := a.Getenv("SODA_PERSON_PASSWORD")
				if password == "" {
					return nil, errors.New("SODA_PERSON_PASSWORD is required")
				}
				return client.CreatePerson(ctx, &sodav2.CreatePersonRequest{Username: username, DisplayName: displayName, Email: email, Role: parsedRole, SshPublicKey: key, Password: password})
			})
		},
	}
	command.Flags().StringVar(&username, "username", "", "Linux username")
	command.Flags().StringVar(&displayName, "display-name", "", "display name")
	command.Flags().StringVar(&email, "email", "", "email address")
	command.Flags().StringVar(&role, "role", "developer", "role: admin or developer")
	command.Flags().StringVar(&keyPath, "ssh-key", "", "SSH public key file")
	_ = command.MarkFlagRequired("username")
	_ = command.MarkFlagRequired("display-name")
	_ = command.MarkFlagRequired("email")
	_ = command.MarkFlagRequired("ssh-key")
	return command
}

func (a *App) projectsCommand(socket *string) *cobra.Command {
	projects := &cobra.Command{Use: "projects", Short: "Manage Soda projects"}
	projects.AddCommand(a.projectListCommand(socket), a.projectCreateCommand(socket), a.collaboratorsCommand(socket), a.worktreesCommand(socket), a.provisioningCommand(socket), a.deployKeyCommand(socket), a.toolchainCommand(socket))
	return projects
}

func (a *App) projectListCommand(socket *string) *cobra.Command {
	return &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
			return client.ListProjects(ctx, &sodav2.ListProjectsRequest{})
		})
	}}
}

func (a *App) projectCreateCommand(socket *string) *cobra.Command {
	var slug, name, profile, remoteURL string
	command := &cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, _ []string) error {
		parsedProfile, err := profileValue(profile)
		if err != nil {
			return err
		}
		source := &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}
		if remoteURL != "" {
			source = &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Git{Git: &sodav2.GitProjectSource{RemoteUrl: remoteURL}}}
		}
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
			return client.CreateProject(ctx, &sodav2.CreateProjectRequest{Slug: slug, Name: name, Profile: parsedProfile, Source: source})
		})
	}}
	command.Flags().StringVar(&slug, "slug", "", "project slug")
	command.Flags().StringVar(&name, "name", "", "project display name")
	command.Flags().StringVar(&profile, "profile", "", "profile: web, python, rust, or go")
	command.Flags().StringVar(&remoteURL, "git", "", "Git remote URL")
	_ = command.MarkFlagRequired("slug")
	_ = command.MarkFlagRequired("name")
	_ = command.MarkFlagRequired("profile")
	return command
}

func (a *App) collaboratorsCommand(socket *string) *cobra.Command {
	var projectID, personID string
	group := &cobra.Command{Use: "collaborators", Short: "Manage project collaborators"}
	add := &cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
			return client.AddCollaborator(ctx, &sodav2.AddCollaboratorRequest{ProjectId: projectID, PersonId: personID})
		})
	}}
	add.Flags().StringVar(&projectID, "project", "", "project ID")
	add.Flags().StringVar(&personID, "person", "", "person ID")
	_ = add.MarkFlagRequired("project")
	_ = add.MarkFlagRequired("person")
	var listProjectID string
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
			return client.ListCollaborators(ctx, &sodav2.ListCollaboratorsRequest{ProjectId: listProjectID})
		})
	}}
	list.Flags().StringVar(&listProjectID, "project", "", "project ID")
	_ = list.MarkFlagRequired("project")
	group.AddCommand(add, list)
	return group
}

func (a *App) worktreesCommand(socket *string) *cobra.Command {
	group := &cobra.Command{Use: "worktrees", Short: "Manage project worktrees"}
	var listProjectID string
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
			return client.ListWorktrees(ctx, &sodav2.ListWorktreesRequest{ProjectId: listProjectID})
		})
	}}
	list.Flags().StringVar(&listProjectID, "project", "", "project ID")
	_ = list.MarkFlagRequired("project")
	var projectID, personID, name, baseRef string
	add := &cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
			return client.CreateWorktree(ctx, &sodav2.CreateWorktreeRequest{ProjectId: projectID, PersonId: personID, Name: name, BaseRef: baseRef})
		})
	}}
	add.Flags().StringVar(&projectID, "project", "", "project ID")
	add.Flags().StringVar(&personID, "person", "", "person ID")
	add.Flags().StringVar(&name, "name", "", "worktree name")
	add.Flags().StringVar(&baseRef, "base", "main", "base Git ref")
	_ = add.MarkFlagRequired("project")
	_ = add.MarkFlagRequired("person")
	_ = add.MarkFlagRequired("name")
	group.AddCommand(list, add)
	return group
}

func (a *App) provisioningCommand(socket *string) *cobra.Command {
	group := &cobra.Command{Use: "provisioning", Short: "Inspect and retry provisioning"}
	var listProjectID string
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
			return client.ListProvisioningJobs(ctx, &sodav2.ListProvisioningJobsRequest{ProjectId: listProjectID})
		})
	}}
	list.Flags().StringVar(&listProjectID, "project", "", "project ID")
	_ = list.MarkFlagRequired("project")
	var retryProjectID string
	retry := &cobra.Command{Use: "retry", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
			return client.StartProvisioning(ctx, &sodav2.StartProvisioningRequest{ProjectId: retryProjectID})
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
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
			return client.GetDeployKey(ctx, &sodav2.GetDeployKeyRequest{ProjectId: projectID})
		})
	}}
	command.Flags().StringVar(&projectID, "project", "", "project ID")
	_ = command.MarkFlagRequired("project")
	return command
}

func (a *App) toolchainCommand(socket *string) *cobra.Command {
	var projectID string
	command := &cobra.Command{Use: "toolchain", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (proto.Message, error) {
			return client.GetProjectToolchain(ctx, &sodav2.GetProjectToolchainRequest{ProjectId: projectID})
		})
	}}
	command.Flags().StringVar(&projectID, "project", "", "project ID")
	_ = command.MarkFlagRequired("project")
	return command
}

func (a *App) call(command *cobra.Command, socket string, operation func(context.Context, sodav2.SodaServiceClient) (proto.Message, error)) error {
	client, closer, err := a.Dial(command.Context(), socket)
	if err != nil {
		return fmt.Errorf("connect to sodad: %w", err)
	}
	defer closer.Close()
	response, err := operation(command.Context(), client)
	if err != nil {
		return canonicalError(err)
	}
	encoded, err := (protojson.MarshalOptions{Indent: "  ", UseProtoNames: true}).Marshal(response)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	_, err = fmt.Fprintln(command.Root().OutOrStdout(), string(encoded))
	return err
}

func (a *App) readKey(path string) (string, error) {
	contents, err := a.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read SSH public key: %w", err)
	}
	return strings.TrimSpace(string(contents)), nil
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
	case codes.Unavailable:
		prefix = "sodad unavailable"
	case codes.DeadlineExceeded:
		prefix = "sodad timed out"
	}
	return fmt.Errorf("%s: %s", prefix, grpcStatus.Message())
}
