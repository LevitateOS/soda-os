package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"path/filepath"
	"strings"
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
	return &sodav2.ListPeopleResponse{People: []*sodav2.Person{{Id: "person-1", Username: "vince", DisplayName: "Vince", Email: "vince@soda.local", Role: sodav2.Role_ROLE_ADMIN}}}, s.record(request)
}

func (s *recordingServer) CreatePerson(_ context.Context, request *sodav2.CreatePersonRequest) (*sodav2.CreatePersonResponse, error) {
	return &sodav2.CreatePersonResponse{Person: &sodav2.Person{Id: "person-1", Username: request.Username, DisplayName: request.DisplayName, Email: request.Email, Role: request.Role}}, s.record(request)
}

func (s *recordingServer) ImportPerson(_ context.Context, request *sodav2.ImportPersonRequest) (*sodav2.ImportPersonResponse, error) {
	return &sodav2.ImportPersonResponse{Person: &sodav2.Person{Id: "person-1", Username: request.Username, DisplayName: request.DisplayName, Email: request.Email, Role: request.Role}}, s.record(request)
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

func (s *recordingServer) GetOSUpdateStatus(_ context.Context, request *sodav2.GetOSUpdateStatusRequest) (*sodav2.GetOSUpdateStatusResponse, error) {
	return &sodav2.GetOSUpdateStatusResponse{Status: testOSUpdateStatus()}, s.record(request)
}

func (s *recordingServer) CheckOSUpdate(_ context.Context, request *sodav2.CheckOSUpdateRequest) (*sodav2.CheckOSUpdateResponse, error) {
	return &sodav2.CheckOSUpdateResponse{Release: &sodav2.OSRelease{ImageReference: "ghcr.io/levitateos/soda-os@sha256:" + strings.Repeat("b", 64), Version: "0.3.0", Digest: "sha256:" + strings.Repeat("b", 64), StateSchema: 3, Available: true}}, s.record(request)
}

func (s *recordingServer) StageOSUpdate(_ context.Context, request *sodav2.StageOSUpdateRequest) (*sodav2.StageOSUpdateResponse, error) {
	return &sodav2.StageOSUpdateResponse{Status: testOSUpdateStatus()}, s.record(request)
}

func (s *recordingServer) ActivateOSUpdate(_ context.Context, request *sodav2.ActivateOSUpdateRequest) (*sodav2.ActivateOSUpdateResponse, error) {
	return &sodav2.ActivateOSUpdateResponse{RebootRequested: request.GetConfirmReboot()}, s.record(request)
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

func testOSUpdateStatus() *sodav2.OSUpdateStatus {
	digest := "sha256:" + strings.Repeat("a", 64)
	return &sodav2.OSUpdateStatus{ReadOnly: true, Booted: &sodav2.OSDeployment{ImageReference: "ghcr.io/levitateos/soda-os@" + digest, Version: "0.2.0", Digest: digest, Architecture: "arm64"}}
}

func testApp(t *testing.T, server *recordingServer) (*app, *string) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	sodav2.RegisterSodaServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() { grpcServer.Stop() })
	var socket string
	app := newApp()
	app.dial = func(ctx context.Context, path string) (sodav2.SodaServiceClient, io.Closer, error) {
		socket = path
		conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, nil, err
		}
		return sodav2.NewSodaServiceClient(conn), conn, nil
	}
	app.getenv = func(name string) string {
		if name == "SODA_PERSON_PASSWORD" {
			return "easy-password"
		}
		return ""
	}
	return app, &socket
}

func execute(t *testing.T, app *app, args ...string) (string, error) {
	t.Helper()
	root := app.command()
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
		{"people add", []string{"people", "add", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local", "--role", "admin"}, func(t *testing.T, got any) {
			request := got.(*sodav2.CreatePersonRequest)
			require.Equal(t, sodav2.Role_ROLE_ADMIN, request.Role)
			require.Equal(t, "easy-password", request.Password)
		}},
		{"people import", []string{"people", "import", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ImportPersonRequest{}, got) }},
		{"projects list", []string{"projects", "list"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ListProjectsRequest{}, got) }},
		{"projects create empty", []string{"projects", "create", "--slug", "demo", "--name", "Demo", "--profile", "go", "--member", "person-1", "--member", "person-2"}, func(t *testing.T, got any) {
			request := got.(*sodav2.CreateProjectRequest)
			require.Equal(t, sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO, request.Profile)
			require.NotNil(t, request.Source.GetEmpty())
			require.Equal(t, []string{"person-1", "person-2"}, request.InitialPersonIds)
		}},
		{"projects create git", []string{"projects", "create", "--slug", "demo", "--name", "Demo", "--profile", "rust", "--git", "ssh://git@example/demo.git", "--member", "person-1"}, func(t *testing.T, got any) {
			require.Equal(t, "ssh://git@example/demo.git", got.(*sodav2.CreateProjectRequest).Source.GetGit().RemoteUrl)
		}},
		{"members add", []string{"projects", "members", "add", "--project", "project", "--person", "person"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.AddCollaboratorRequest{}, got) }},
		{"members list", []string{"projects", "members", "list", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ListCollaboratorsRequest{}, got) }},
		{"workspaces list", []string{"projects", "workspaces", "list", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ListWorktreesRequest{}, got) }},
		{"provisioning retry", []string{"projects", "provisioning", "retry", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.StartProvisioningRequest{}, got) }},
		{"provisioning list", []string{"projects", "provisioning", "list", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.ListProvisioningJobsRequest{}, got) }},
		{"deploy key", []string{"projects", "deploy-key", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.GetDeployKeyRequest{}, got) }},
		{"toolchain", []string{"projects", "toolchain", "--project", "project"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.GetProjectToolchainRequest{}, got) }},
		{"os update status", []string{"os", "update", "status"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.GetOSUpdateStatusRequest{}, got) }},
		{"os update check", []string{"os", "update", "check"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.CheckOSUpdateRequest{}, got) }},
		{"os update stage", []string{"os", "update", "stage"}, func(t *testing.T, got any) {
			request := got.(*sodav2.StageOSUpdateRequest)
			require.Equal(t, "ghcr.io/levitateos/soda-os@sha256:"+strings.Repeat("b", 64), request.GetImageReference())
		}},
		{"os update activate", []string{"os", "update", "activate", "--confirm-reboot"}, func(t *testing.T, got any) {
			require.True(t, got.(*sodav2.ActivateOSUpdateRequest).GetConfirmReboot())
		}},
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

func TestOSActivationRequiresExplicitConfirmationFlagBeforeCallingDaemon(t *testing.T) {
	app := newApp()
	dials := 0
	app.dial = func(context.Context, string) (sodav2.SodaServiceClient, io.Closer, error) {
		dials++
		return nil, nil, io.ErrUnexpectedEOF
	}
	_, err := execute(t, app, "os", "update", "activate")
	require.ErrorContains(t, err, "required flag(s) \"confirm-reboot\" not set")
	require.Zero(t, dials)
}

func TestPersonAddRequiresPasswordBeforeCallingDaemon(t *testing.T) {
	app := newApp()
	dials := 0
	app.dial = func(context.Context, string) (sodav2.SodaServiceClient, io.Closer, error) {
		dials++
		return nil, nil, io.ErrUnexpectedEOF
	}
	app.getenv = func(string) string { return "" }
	_, err := execute(t, app, "people", "add", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local")
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
	app := newApp()
	dials := 0
	app.dial = func(context.Context, string) (sodav2.SodaServiceClient, io.Closer, error) {
		dials++
		return nil, nil, io.ErrUnexpectedEOF
	}
	_, err := execute(t, app, "projects", "create", "--slug", "demo", "--name", "Demo", "--profile", "invalid", "--member", "person-1")
	require.EqualError(t, err, "invalid profile \"invalid\"; expected web, python, rust, or go")
	require.Zero(t, dials)

	_, err = execute(t, app, "projects", "create", "--slug", "demo", "--name", "Demo", "--profile", "go")
	require.ErrorContains(t, err, "required flag(s) \"member\" not set")
	require.Zero(t, dials)
}

func TestMissingSocketReturnsUnavailableWithinDeadline(t *testing.T) {
	app := newApp()
	app.timeout = 100 * time.Millisecond
	socket := filepath.Join(t.TempDir(), "missing.sock")
	started := time.Now()
	_, err := execute(t, app, "--socket", socket, "health")
	require.EqualError(t, err, "sodad unavailable: Soda service is unavailable")
	require.Less(t, time.Since(started), time.Second)
}

func TestJSONShapes(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{"health", []string{"health"}, `{"status":"ready","service":"sodad","version":"0.2.0"}`},
		{"people list is array", []string{"people", "list"}, `[{"id":"person-1","username":"vince","display_name":"Vince","email":"vince@soda.local","role":"admin"}]`},
		{"person is unwrapped", []string{"people", "add", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local", "--role", "admin"}, `{"id":"person-1","username":"vince","display_name":"Vince","email":"vince@soda.local","role":"admin"}`},
		{"imported person is unwrapped", []string{"people", "import", "--username", "vince", "--display-name", "Vince", "--email", "vince@soda.local"}, `{"id":"person-1","username":"vince","display_name":"Vince","email":"vince@soda.local","role":"developer"}`},
		{"projects list is array", []string{"projects", "list"}, `[{"id":"project-1","slug":"demo","name":"Demo","unix_user":"soda-p-demo","profile":"go","source":{"kind":"empty"}}]`},
		{"project is unwrapped", []string{"projects", "create", "--slug", "demo", "--name", "Demo", "--profile", "rust", "--git", "ssh://git@example/demo.git", "--member", "person-1"}, `{"id":"project-1","slug":"demo","name":"Demo","unix_user":"soda-p-demo","profile":"rust","source":{"kind":"git","remote_url":"ssh://git@example/demo.git"}}`},
		{"member add returns workspace", []string{"projects", "members", "add", "--project", "project", "--person", "person"}, `{"id":"worktree-1","project_id":"project","person_id":"person","name":"feature","branch":"people/vince","path":"/srv/soda/projects/demo/worktrees/vince"}`},
		{"members list is array", []string{"projects", "members", "list", "--project", "project"}, `[{"person":{"id":"person-1","username":"vince","display_name":"","email":"","role":"admin"},"membership":{"project_id":"project","person_id":"person-1"},"workspaces":[{"id":"worktree-1","project_id":"project","person_id":"person","name":"feature","branch":"people/vince","path":"/srv/soda/projects/demo/worktrees/vince"}]}]`},
		{"workspaces list is array", []string{"projects", "workspaces", "list", "--project", "project"}, `[{"id":"worktree-1","project_id":"project","person_id":"person","name":"feature","branch":"people/vince","path":"/srv/soda/projects/demo/worktrees/vince"}]`},
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
