package daemonclient

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeSodaService struct {
	sodav2.UnimplementedSodaServiceServer
	projectRequest *sodav2.CreateProjectRequest
	activation     *sodav2.ActivateOSUpdateRequest
}

func (s *fakeSodaService) ActivateOSUpdate(_ context.Context, request *sodav2.ActivateOSUpdateRequest) (*sodav2.ActivateOSUpdateResponse, error) {
	s.activation = request
	return &sodav2.ActivateOSUpdateResponse{}, nil
}

func (s *fakeSodaService) ListPeople(context.Context, *sodav2.ListPeopleRequest) (*sodav2.ListPeopleResponse, error) {
	return &sodav2.ListPeopleResponse{People: []*sodav2.Person{{Id: "person-1", Username: "alice", DisplayName: "Alice", Email: "alice@soda.local", Role: sodav2.Role_ROLE_ADMIN}}}, nil
}

func (s *fakeSodaService) ListSshDeviceKeys(context.Context, *sodav2.ListSshDeviceKeysRequest) (*sodav2.ListSshDeviceKeysResponse, error) {
	return &sodav2.ListSshDeviceKeysResponse{Keys: []*sodav2.SshDeviceKey{{Id: "key-1", PersonId: "person-1", Label: "Laptop", PublicKey: "ssh-ed25519 AAAA", Fingerprint: "SHA256:test", IdentityFileHint: "~/.ssh/id_ed25519", CreatedAt: timestamppb.New(time.Unix(1_700_000_000, 0))}}}, nil
}

func (s *fakeSodaService) ListCollaborators(context.Context, *sodav2.ListCollaboratorsRequest) (*sodav2.ListCollaboratorsResponse, error) {
	return &sodav2.ListCollaboratorsResponse{Collaborators: []*sodav2.Collaborator{{Person: &sodav2.Person{Id: "person-1", Username: "alice", DisplayName: "Alice", Role: sodav2.Role_ROLE_ADMIN}}}}, nil
}

func (s *fakeSodaService) CreateProject(_ context.Context, request *sodav2.CreateProjectRequest) (*sodav2.CreateProjectResponse, error) {
	s.projectRequest = request
	return &sodav2.CreateProjectResponse{Project: &sodav2.Project{Id: "project-1", Slug: request.GetSlug(), Name: request.GetName(), UnixUser: "soda-p-" + request.GetSlug(), Profile: request.GetProfile(), Source: request.GetSource()}}, nil
}

func (*fakeSodaService) GetProjectToolchain(context.Context, *sodav2.GetProjectToolchainRequest) (*sodav2.GetProjectToolchainResponse, error) {
	return nil, status.Error(codes.NotFound, "toolchain not resolved")
}

func (*fakeSodaService) GetHostStatus(context.Context, *sodav2.GetHostStatusRequest) (*sodav2.GetHostStatusResponse, error) {
	cpu := 24.5
	return &sodav2.GetHostStatusResponse{Host: &sodav2.HostStatus{SampledAt: timestamppb.New(time.Unix(1_700_000_000, 0)), Overall: sodav2.RuntimeState_RUNTIME_STATE_READY, CpuPercent: &cpu, LoadAverage: &sodav2.LoadAverage{OneMinute: 1, FiveMinutes: 2, FifteenMinutes: 3}}}, nil
}

func TestClientMapsPeopleAndCollaborators(t *testing.T) {
	client, _ := bufconnClient(t)
	people, err := client.People(context.Background())
	if err != nil || len(people) != 1 {
		t.Fatalf("People() = %#v, %v", people, err)
	}
	if people[0].Role != RoleAdmin || people[0].Username != "alice" {
		t.Fatalf("People() = %#v", people)
	}
	members, err := client.Members(context.Background(), "project-1")
	if err != nil || len(members) != 1 {
		t.Fatalf("Members() = %#v, %v", members, err)
	}
	if members[0].Username != "alice" {
		t.Fatalf("Members() = %#v", members)
	}
}

func TestClientMapsSSHDeviceKeys(t *testing.T) {
	client, _ := bufconnClient(t)
	keys, err := client.SSHDeviceKeys(context.Background(), "person-1")
	if err != nil || len(keys) != 1 {
		t.Fatalf("SSHDeviceKeys() = %#v, %v", keys, err)
	}
	got := struct {
		Label     string
		Type      string
		CreatedAt int64
	}{Label: keys[0].Label, Type: strings.Fields(keys[0].PublicKey)[0], CreatedAt: keys[0].CreatedAt.Unix()}
	want := struct {
		Label     string
		Type      string
		CreatedAt int64
	}{Label: "Laptop", Type: "ssh-ed25519", CreatedAt: 1_700_000_000}
	if got != want {
		t.Fatalf("SSHDeviceKeys() = %#v", keys)
	}
}

func TestClientMapsProjectRequestAndResponse(t *testing.T) {
	client, service := bufconnClient(t)
	project, err := client.CreateProject(context.Background(), CreateProjectRequest{Slug: "demo", Name: "Demo", Profile: "go", Source: GitProjectSource{RemoteURL: "ssh://git@example/demo.git"}, InitialPersonIDs: []string{"person-1"}})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	gitSource, gitProject := project.Source.(GitProjectSource)
	if service.projectRequest.GetSource().GetGit().GetRemoteUrl() != "ssh://git@example/demo.git" || len(service.projectRequest.GetInitialPersonIds()) != 1 || project.Profile != "go" || !gitProject || gitSource.RemoteURL != "ssh://git@example/demo.git" {
		t.Fatalf("project mapping failed: request=%#v result=%#v", service.projectRequest, project)
	}
}

func TestClientMapsOptionalHostValues(t *testing.T) {
	client, _ := bufconnClient(t)
	host, err := client.HostStatus(context.Background())
	if err != nil || host.Resources.CPUPercent == nil {
		t.Fatalf("HostStatus() = %#v, %v", host, err)
	}
	got := struct {
		Overall RuntimeState
		CPU     float64
		Load    [3]float64
	}{Overall: host.Health.Overall, CPU: *host.Resources.CPUPercent, Load: host.Resources.LoadAverage}
	want := struct {
		Overall RuntimeState
		CPU     float64
		Load    [3]float64
	}{Overall: "ready", CPU: 24.5, Load: [3]float64{1, 2, 3}}
	if got != want {
		t.Fatalf("HostStatus() = %#v", host)
	}
}

func TestClientAlwaysConfirmsActivatedReboot(t *testing.T) {
	client, service := bufconnClient(t)
	if err := client.ActivateOSUpdate(context.Background()); err != nil {
		t.Fatalf("ActivateOSUpdate() error = %v", err)
	}
	if service.activation == nil || !service.activation.GetConfirmReboot() {
		t.Fatalf("ActivateOSUpdate() request = %#v", service.activation)
	}
}

func TestClientPreservesMissingToolchain(t *testing.T) {
	client, _ := bufconnClient(t)
	installation, err := client.Toolchain(context.Background(), "project-1")
	if err != nil || installation != nil {
		t.Fatalf("Toolchain() = %#v, %v; want nil, nil", installation, err)
	}
}

func TestClientSanitizesGRPCErrors(t *testing.T) {
	tests := []struct {
		name    string
		code    codes.Code
		detail  string
		message string
	}{
		{name: "validation", code: codes.InvalidArgument, detail: "username must start with a lowercase letter", message: "username must start with a lowercase letter"},
		{name: "conflict", code: codes.AlreadyExists, detail: "username already exists", message: "username already exists"},
		{name: "unavailable", code: codes.Unavailable, detail: "dial unix /run/soda/sodad.sock: connection refused", message: "Soda service unavailable."},
		{name: "internal", code: codes.Internal, detail: "sqlite failed at /var/lib/soda/soda.db", message: "Soda service error."},
		{name: "unknown", code: codes.Unknown, detail: "panic: secret implementation detail", message: "Soda service error."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newClient(failingService{err: status.Error(test.code, test.detail)}, nil)
			_, err := client.People(context.Background())
			if err == nil || err.Error() != test.message {
				t.Fatalf("People() error = %#v, want %q", err, test.message)
			}
			if test.message != test.detail && strings.Contains(err.Error(), test.detail) {
				t.Fatalf("People() exposed daemon detail %q", test.detail)
			}
		})
	}
}

func TestNewClientReturnsBeforeDaemonIsAvailable(t *testing.T) {
	directory, err := os.MkdirTemp("/tmp", "soda-cockpit-grpc-")
	if err != nil {
		t.Fatalf("create socket directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	socketPath := filepath.Join(directory, "sodad.sock")
	started := time.Now()
	client, err := NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("NewClient() blocked for %s", elapsed)
	}
}

func TestClientRecoversWhenDaemonSocketAppears(t *testing.T) {
	directory, err := os.MkdirTemp("/tmp", "soda-cockpit-grpc-")
	if err != nil {
		t.Fatalf("create socket directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	socketPath := filepath.Join(directory, "sodad.sock")
	client, err := NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen on recovered socket: %v", err)
	}
	server := grpc.NewServer()
	sodav2.RegisterSodaServiceServer(server, &fakeSodaService{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		people, callErr := client.People(ctx)
		cancel()
		if callErr == nil {
			if len(people) != 1 || people[0].Username != "alice" {
				t.Fatalf("People() after recovery = %#v", people)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("People() did not recover")
}

type failingService struct {
	sodav2.SodaServiceClient
	err error
}

func (service failingService) ListPeople(context.Context, *sodav2.ListPeopleRequest, ...grpc.CallOption) (*sodav2.ListPeopleResponse, error) {
	return nil, service.err
}

func bufconnClient(t *testing.T) (*Client, *fakeSodaService) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	service := &fakeSodaService{}
	sodav2.RegisterSodaServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	connection, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	client := newClient(sodav2.NewSodaServiceClient(connection), connection)
	t.Cleanup(func() { _ = client.Close() })
	return client, service
}
