package sodactl

import (
	"bytes"
	"context"
	"io"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

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
	return &sodav2.ListPeopleResponse{People: []*sodav2.Person{{Id: "person-1", Username: "vince", DisplayName: "Vince", Email: "vince@soda.local", Role: sodav2.Role_ROLE_ADMIN, SshPublicKey: "ssh-ed25519 AAAA"}}}, s.record(request)
}

func (s *recordingServer) CreatePerson(_ context.Context, request *sodav2.CreatePersonRequest) (*sodav2.CreatePersonResponse, error) {
	return &sodav2.CreatePersonResponse{Person: &sodav2.Person{Id: "person-1", Username: request.Username, DisplayName: request.DisplayName, Email: request.Email, Role: request.Role, SshPublicKey: request.SshPublicKey}}, s.record(request)
}

func (s *recordingServer) ImportPerson(_ context.Context, request *sodav2.ImportPersonRequest) (*sodav2.ImportPersonResponse, error) {
	return &sodav2.ImportPersonResponse{Person: &sodav2.Person{Id: "person-1", Username: request.Username, DisplayName: request.DisplayName, Email: request.Email, Role: request.Role, SshPublicKey: request.SshPublicKey}}, s.record(request)
}

func (s *recordingServer) ListProjects(_ context.Context, request *sodav2.ListProjectsRequest) (*sodav2.ListProjectsResponse, error) {
	return &sodav2.ListProjectsResponse{Projects: []*sodav2.Project{{Id: "project-1", Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO, Source: emptySource()}}}, s.record(request)
}

func (s *recordingServer) CreateProject(_ context.Context, request *sodav2.CreateProjectRequest) (*sodav2.CreateProjectResponse, error) {
	return &sodav2.CreateProjectResponse{Project: &sodav2.Project{Id: "project-1", Slug: request.Slug, Name: request.Name, UnixUser: "soda-p-" + request.Slug, Profile: request.Profile, Source: request.Source}}, s.record(request)
}

func (s *recordingServer) AddCollaborator(_ context.Context, request *sodav2.AddCollaboratorRequest) (*sodav2.AddCollaboratorResponse, error) {
	return &sodav2.AddCollaboratorResponse{Membership: &sodav2.Membership{ProjectId: request.ProjectId, PersonId: request.PersonId}, Worktree: testWorktree()}, s.record(request)
}

func (s *recordingServer) ListCollaborators(_ context.Context, request *sodav2.ListCollaboratorsRequest) (*sodav2.ListCollaboratorsResponse, error) {
	return &sodav2.ListCollaboratorsResponse{Collaborators: []*sodav2.Collaborator{{Person: &sodav2.Person{Id: "person-1", Username: "vince", Role: sodav2.Role_ROLE_ADMIN}, Membership: &sodav2.Membership{ProjectId: request.ProjectId, PersonId: "person-1"}, Worktrees: []*sodav2.Worktree{testWorktree()}}}}, s.record(request)
}

func (s *recordingServer) CreateWorktree(_ context.Context, request *sodav2.CreateWorktreeRequest) (*sodav2.CreateWorktreeResponse, error) {
	return &sodav2.CreateWorktreeResponse{Worktree: testWorktree()}, s.record(request)
}

func (s *recordingServer) ListWorktrees(_ context.Context, request *sodav2.ListWorktreesRequest) (*sodav2.ListWorktreesResponse, error) {
	return &sodav2.ListWorktreesResponse{Worktrees: []*sodav2.Worktree{testWorktree()}}, s.record(request)
}

func (s *recordingServer) ListProvisioningJobs(_ context.Context, request *sodav2.ListProvisioningJobsRequest) (*sodav2.ListProvisioningJobsResponse, error) {
	return &sodav2.ListProvisioningJobsResponse{Jobs: []*sodav2.ProvisioningJob{testJob()}}, s.record(request)
}

func (s *recordingServer) StartProvisioning(_ context.Context, request *sodav2.StartProvisioningRequest) (*sodav2.StartProvisioningResponse, error) {
	return &sodav2.StartProvisioningResponse{Job: testJob()}, s.record(request)
}

func (s *recordingServer) GetDeployKey(_ context.Context, request *sodav2.GetDeployKeyRequest) (*sodav2.GetDeployKeyResponse, error) {
	return &sodav2.GetDeployKeyResponse{DeployKey: &sodav2.DeployKey{ProjectId: request.ProjectId, PublicKey: "ssh-ed25519 DEPLOY"}}, s.record(request)
}

func (s *recordingServer) GetProjectToolchain(_ context.Context, request *sodav2.GetProjectToolchainRequest) (*sodav2.GetProjectToolchainResponse, error) {
	return &sodav2.GetProjectToolchainResponse{Installation: &sodav2.ToolchainInstallation{Id: "toolchain-1", Profile: sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO, Version: "1.25.1", Path: "/opt/go", Checksum: "abc", State: sodav2.JobState_JOB_STATE_READY}, Resolution: &sodav2.ProjectToolchainResolution{ProjectId: request.ProjectId, ToolchainInstallationId: "toolchain-1"}}, s.record(request)
}

func emptySource() *sodav2.ProjectSource {
	return &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}
}

func testWorktree() *sodav2.Worktree {
	return &sodav2.Worktree{Id: "worktree-1", ProjectId: "project", PersonId: "person", Name: "feature", Branch: "people/vince", Path: "/srv/soda/projects/demo/worktrees/vince"}
}

func testJob() *sodav2.ProvisioningJob {
	return &sodav2.ProvisioningJob{Id: "job-1", ProjectId: "project", State: sodav2.JobState_JOB_STATE_INSTALLING}
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
	app := New()
	dials := 0
	app.Dial = func(context.Context, string) (sodav2.SodaServiceClient, io.Closer, error) {
		dials++
		return nil, nil, io.ErrUnexpectedEOF
	}
	app.Getenv = func(string) string { return "" }
	_, err := execute(t, app, "people", "add", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local", "--ssh-key", "id.pub")
	require.EqualError(t, err, "SODA_PERSON_PASSWORD is required")
	require.Zero(t, dials)
}

func TestCanonicalGRPCErrors(t *testing.T) {
	t.Run("user actionable messages remain visible", func(t *testing.T) {
		server := &recordingServer{err: status.Error(codes.NotFound, "project missing")}
		app, _ := testApp(t, server)
		_, err := execute(t, app, "projects", "deploy-key", "--project", "missing")
		require.EqualError(t, err, "not found: project missing")
	})
	for _, code := range []codes.Code{codes.Internal, codes.Unknown} {
		t.Run(code.String(), func(t *testing.T) {
			server := &recordingServer{err: status.Error(code, "SQL failure at /var/lib/soda/soda.db: secret")}
			app, _ := testApp(t, server)
			_, err := execute(t, app, "projects", "deploy-key", "--project", "project")
			require.EqualError(t, err, "sodad error: internal service error")
			require.NotContains(t, err.Error(), "SQL")
			require.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestLocalValidationFailureNeverDials(t *testing.T) {
	app := New()
	dials := 0
	app.Dial = func(context.Context, string) (sodav2.SodaServiceClient, io.Closer, error) {
		dials++
		return nil, nil, io.ErrUnexpectedEOF
	}
	_, err := execute(t, app, "projects", "create", "--slug", "demo", "--name", "Demo", "--profile", "invalid")
	require.EqualError(t, err, "invalid profile \"invalid\"; expected web, python, rust, or go")
	require.Zero(t, dials)

	app.Getenv = func(string) string { return "password" }
	app.ReadFile = func(string) ([]byte, error) { return nil, io.ErrUnexpectedEOF }
	_, err = execute(t, app, "people", "import", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local", "--ssh-key", "missing")
	require.ErrorContains(t, err, "read SSH public key")
	require.Zero(t, dials)
}

func TestMissingSocketReturnsUnavailableWithinDeadline(t *testing.T) {
	app := New()
	app.Timeout = 100 * time.Millisecond
	socket := filepath.Join(t.TempDir(), "missing.sock")
	started := time.Now()
	_, err := execute(t, app, "--socket", socket, "health")
	require.EqualError(t, err, "sodad unavailable: Soda service is unavailable")
	require.Less(t, time.Since(started), time.Second)
}

func TestLegacyJSONShapes(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{"health", []string{"health"}, `{"status":"ready","service":"sodad","version":"0.2.0"}`},
		{"people list is array", []string{"people", "list"}, `[{"id":"person-1","username":"vince","display_name":"Vince","email":"vince@soda.local","role":"admin","ssh_public_key":"ssh-ed25519 AAAA"}]`},
		{"person is unwrapped", []string{"people", "add", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local", "--role", "admin", "--ssh-key", "id.pub"}, `{"id":"person-1","username":"vince","display_name":"Vince","email":"vince@soda.local","role":"admin","ssh_public_key":"ssh-ed25519 AAAA test"}`},
		{"imported person is unwrapped", []string{"people", "import", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local", "--ssh-key", "id.pub"}, `{"id":"person-1","username":"vince","display_name":"Vince","email":"vince@soda.local","role":"developer","ssh_public_key":"ssh-ed25519 AAAA test"}`},
		{"projects list is array", []string{"projects", "list"}, `[{"id":"project-1","slug":"demo","name":"Demo","unix_user":"soda-p-demo","profile":"go","source":{"kind":"empty"}}]`},
		{"project is unwrapped", []string{"projects", "create", "--slug", "demo", "--name", "Demo", "--profile", "rust", "--git", "ssh://git@example/demo.git"}, `{"id":"project-1","slug":"demo","name":"Demo","unix_user":"soda-p-demo","profile":"rust","source":{"kind":"git","remote_url":"ssh://git@example/demo.git"}}`},
		{"collaborator add returns worktree", []string{"projects", "collaborators", "add", "--project", "project", "--person", "person"}, `{"id":"worktree-1","project_id":"project","person_id":"person","name":"feature","branch":"people/vince","path":"/srv/soda/projects/demo/worktrees/vince"}`},
		{"collaborators list is array", []string{"projects", "collaborators", "list", "--project", "project"}, `[{"person":{"id":"person-1","username":"vince","display_name":"","email":"","role":"admin","ssh_public_key":""},"membership":{"project_id":"project","person_id":"person-1"},"worktrees":[{"id":"worktree-1","project_id":"project","person_id":"person","name":"feature","branch":"people/vince","path":"/srv/soda/projects/demo/worktrees/vince"}]}]`},
		{"worktree add is unwrapped", []string{"projects", "worktrees", "add", "--project", "project", "--person", "person", "--name", "feature"}, `{"id":"worktree-1","project_id":"project","person_id":"person","name":"feature","branch":"people/vince","path":"/srv/soda/projects/demo/worktrees/vince"}`},
		{"worktrees list is array", []string{"projects", "worktrees", "list", "--project", "project"}, `[{"id":"worktree-1","project_id":"project","person_id":"person","name":"feature","branch":"people/vince","path":"/srv/soda/projects/demo/worktrees/vince"}]`},
		{"retry returns job", []string{"projects", "provisioning", "retry", "--project", "project"}, `{"id":"job-1","project_id":"project","state":"installing","error":null}`},
		{"jobs list is array", []string{"projects", "provisioning", "list", "--project", "project"}, `[{"id":"job-1","project_id":"project","state":"installing","error":null}]`},
		{"deploy key is unwrapped", []string{"projects", "deploy-key", "--project", "project"}, `{"project_id":"project","public_key":"ssh-ed25519 DEPLOY"}`},
		{"toolchain is unwrapped", []string{"projects", "toolchain", "--project", "project"}, `{"id":"toolchain-1","profile":"go","version":"1.25.1","path":"/opt/go","checksum":"abc","state":"ready"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, _ := testApp(t, &recordingServer{})
			output, err := execute(t, app, test.args...)
			require.NoError(t, err)
			require.JSONEq(t, test.expected, output)
		})
	}
}
