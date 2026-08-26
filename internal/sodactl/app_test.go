package sodactl

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type recordingServer struct {
	sodav2.UnimplementedSodaServiceServer
	mu  sync.Mutex
	got any
	err error
}

func (s *recordingServer) record(request any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = request
	return s.err
}

func (s *recordingServer) Health(_ context.Context, request *sodav2.HealthRequest) (*sodav2.HealthResponse, error) {
	return &sodav2.HealthResponse{Status: "ready", Service: "sodad", Version: "0.2.0"}, s.record(request)
}

func (s *recordingServer) ListPeople(_ context.Context, request *sodav2.ListPeopleRequest) (*sodav2.ListPeopleResponse, error) {
	return &sodav2.ListPeopleResponse{People: []*sodav2.Person{{Username: "vince"}}}, s.record(request)
}

func (s *recordingServer) CreatePerson(_ context.Context, request *sodav2.CreatePersonRequest) (*sodav2.CreatePersonResponse, error) {
	return &sodav2.CreatePersonResponse{Person: &sodav2.Person{Username: request.Username}}, s.record(request)
}

func (s *recordingServer) ImportPerson(_ context.Context, request *sodav2.ImportPersonRequest) (*sodav2.ImportPersonResponse, error) {
	return &sodav2.ImportPersonResponse{Person: &sodav2.Person{Username: request.Username}}, s.record(request)
}

func (s *recordingServer) ListProjects(_ context.Context, request *sodav2.ListProjectsRequest) (*sodav2.ListProjectsResponse, error) {
	return &sodav2.ListProjectsResponse{Projects: []*sodav2.Project{{Slug: "demo"}}}, s.record(request)
}

func (s *recordingServer) CreateProject(_ context.Context, request *sodav2.CreateProjectRequest) (*sodav2.CreateProjectResponse, error) {
	return &sodav2.CreateProjectResponse{Project: &sodav2.Project{Slug: request.Slug}}, s.record(request)
}

func (s *recordingServer) AddCollaborator(_ context.Context, request *sodav2.AddCollaboratorRequest) (*sodav2.AddCollaboratorResponse, error) {
	return &sodav2.AddCollaboratorResponse{}, s.record(request)
}

func (s *recordingServer) ListCollaborators(_ context.Context, request *sodav2.ListCollaboratorsRequest) (*sodav2.ListCollaboratorsResponse, error) {
	return &sodav2.ListCollaboratorsResponse{}, s.record(request)
}

func (s *recordingServer) CreateWorktree(_ context.Context, request *sodav2.CreateWorktreeRequest) (*sodav2.CreateWorktreeResponse, error) {
	return &sodav2.CreateWorktreeResponse{}, s.record(request)
}

func (s *recordingServer) ListWorktrees(_ context.Context, request *sodav2.ListWorktreesRequest) (*sodav2.ListWorktreesResponse, error) {
	return &sodav2.ListWorktreesResponse{}, s.record(request)
}

func (s *recordingServer) ListProvisioningJobs(_ context.Context, request *sodav2.ListProvisioningJobsRequest) (*sodav2.ListProvisioningJobsResponse, error) {
	return &sodav2.ListProvisioningJobsResponse{}, s.record(request)
}

func (s *recordingServer) StartProvisioning(_ context.Context, request *sodav2.StartProvisioningRequest) (*sodav2.StartProvisioningResponse, error) {
	return &sodav2.StartProvisioningResponse{}, s.record(request)
}

func (s *recordingServer) GetDeployKey(_ context.Context, request *sodav2.GetDeployKeyRequest) (*sodav2.GetDeployKeyResponse, error) {
	return &sodav2.GetDeployKeyResponse{DeployKey: &sodav2.DeployKey{ProjectId: request.ProjectId}}, s.record(request)
}

func (s *recordingServer) GetProjectToolchain(_ context.Context, request *sodav2.GetProjectToolchainRequest) (*sodav2.GetProjectToolchainResponse, error) {
	return &sodav2.GetProjectToolchainResponse{}, s.record(request)
}

func testApp(t *testing.T, server *recordingServer) (*App, *string) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	sodav2.RegisterSodaServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() { grpcServer.Stop() })
	var socket string
	app := New()
	app.Dial = func(ctx context.Context, path string) (sodav2.SodaServiceClient, io.Closer, error) {
		socket = path
		conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, nil, err
		}
		return sodav2.NewSodaServiceClient(conn), conn, nil
	}
	app.Getenv = func(name string) string {
		if name == "SODA_PERSON_PASSWORD" {
			return "easy-password"
		}
		return ""
	}
	app.ReadFile = func(string) ([]byte, error) { return []byte(" ssh-ed25519 AAAA test\n"), nil }
	return app, &socket
}

func execute(t *testing.T, app *App, args ...string) (string, error) {
	t.Helper()
	root := app.Command()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)
	err := root.Execute()
	return output.String(), err
}

func TestCommandsUseGRPCAndWriteSnakeCaseJSON(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(t *testing.T, got any)
	}{
		{"health", []string{"--socket", "/tmp/soda.sock", "health"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.HealthRequest{}, got) }},
		{"people list", []string{"people", "list"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ListPeopleRequest{}, got) }},
		{"people add", []string{"people", "add", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local", "--role", "admin", "--ssh-key", "id.pub"}, func(t *testing.T, got any) {
			request := got.(*sodav2.CreatePersonRequest)
			require.Equal(t, sodav2.Role_ROLE_ADMIN, request.Role)
			require.Equal(t, "ssh-ed25519 AAAA test", request.SshPublicKey)
			require.Equal(t, "easy-password", request.Password)
		}},
		{"people import", []string{"people", "import", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local", "--ssh-key", "id.pub"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ImportPersonRequest{}, got) }},
		{"projects list", []string{"projects", "list"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ListProjectsRequest{}, got) }},
		{"projects create empty", []string{"projects", "create", "--slug", "demo", "--name", "Demo", "--profile", "go"}, func(t *testing.T, got any) {
			request := got.(*sodav2.CreateProjectRequest)
			require.Equal(t, sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO, request.Profile)
			require.NotNil(t, request.Source.GetEmpty())
		}},
		{"projects create git", []string{"projects", "create", "--slug", "demo", "--name", "Demo", "--profile", "rust", "--git", "ssh://git@example/demo.git"}, func(t *testing.T, got any) {
			require.Equal(t, "ssh://git@example/demo.git", got.(*sodav2.CreateProjectRequest).Source.GetGit().RemoteUrl)
		}},
		{"collaborators add", []string{"projects", "collaborators", "add", "--project", "project", "--person", "person"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.AddCollaboratorRequest{}, got) }},
		{"collaborators list", []string{"projects", "collaborators", "list", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ListCollaboratorsRequest{}, got) }},
		{"worktrees add", []string{"projects", "worktrees", "add", "--project", "project", "--person", "person", "--name", "feature", "--base", "main"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.CreateWorktreeRequest{}, got) }},
		{"worktrees list", []string{"projects", "worktrees", "list", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ListWorktreesRequest{}, got) }},
		{"provisioning retry", []string{"projects", "provisioning", "retry", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.StartProvisioningRequest{}, got) }},
		{"provisioning list", []string{"projects", "provisioning", "list", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ListProvisioningJobsRequest{}, got) }},
		{"deploy key", []string{"projects", "deploy-key", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.GetDeployKeyRequest{}, got) }},
		{"toolchain", []string{"projects", "toolchain", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.GetProjectToolchainRequest{}, got) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &recordingServer{}
			app, socket := testApp(t, server)
			output, err := execute(t, app, test.args...)
			require.NoError(t, err)
			require.Contains(t, output, "\n")
			test.check(t, server.got)
			if test.name == "health" {
				require.Equal(t, "/tmp/soda.sock", *socket)
				require.Contains(t, output, "\"service\":")
				require.Contains(t, output, "sodad")
			}
		})
	}
}

func TestPersonAddRequiresPasswordBeforeCallingDaemon(t *testing.T) {
	server := &recordingServer{}
	app, _ := testApp(t, server)
	app.Getenv = func(string) string { return "" }
	_, err := execute(t, app, "people", "add", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local", "--ssh-key", "id.pub")
	require.EqualError(t, err, "SODA_PERSON_PASSWORD is required")
	require.Nil(t, server.got)
}

func TestCanonicalGRPCErrors(t *testing.T) {
	server := &recordingServer{err: status.Error(codes.NotFound, "project missing")}
	app, _ := testApp(t, server)
	_, err := execute(t, app, "projects", "deploy-key", "--project", "missing")
	require.EqualError(t, err, "not found: project missing")
	require.True(t, strings.Contains(err.Error(), "not found"))
}

func TestInvalidValuesAndKeyReadErrors(t *testing.T) {
	server := &recordingServer{}
	app, _ := testApp(t, server)
	_, err := execute(t, app, "projects", "create", "--slug", "demo", "--name", "Demo", "--profile", "invalid")
	require.EqualError(t, err, "invalid profile \"invalid\"; expected web, python, rust, or go")
	app.ReadFile = func(string) ([]byte, error) { return nil, io.ErrUnexpectedEOF }
	_, err = execute(t, app, "people", "import", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local", "--ssh-key", "missing")
	require.ErrorContains(t, err, "read SSH public key")
}
