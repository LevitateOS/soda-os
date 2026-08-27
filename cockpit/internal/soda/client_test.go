package soda

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
	eventProjectID *string
	invalidControl bool
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

func (s *fakeSodaService) SubscribeEvents(request *sodav2.SubscribeEventsRequest, stream sodav2.SodaService_SubscribeEventsServer) error {
	s.eventProjectID = request.ProjectId
	if s.invalidControl {
		return stream.Send(&sodav2.SubscribeEventsResponse{Payload: &sodav2.SubscribeEventsResponse_Control{Control: sodav2.StreamControl_STREAM_CONTROL_UNSPECIFIED}})
	}
	if err := stream.Send(&sodav2.SubscribeEventsResponse{Payload: &sodav2.SubscribeEventsResponse_Control{Control: sodav2.StreamControl_STREAM_CONTROL_REFRESH}}); err != nil {
		return err
	}
	projectID := "project-1"
	return stream.Send(&sodav2.SubscribeEventsResponse{Payload: &sodav2.SubscribeEventsResponse_Event{Event: &sodav2.SodaEvent{Kind: sodav2.EventKind_EVENT_KIND_GIT_CHANGED, ProjectId: &projectID, Sequence: 9}}})
}

func TestClientMapsGRPCResourcesAndOptionalValues(t *testing.T) {
	client, service := bufconnClient(t)
	people, err := client.People(context.Background())
	if err != nil {
		t.Fatalf("People() error = %v", err)
	}
	if len(people) != 1 || people[0].Role != RoleAdmin || people[0].Username != "alice" {
		t.Fatalf("People() = %#v", people)
	}
	keys, err := client.SSHDeviceKeys(context.Background(), "person-1")
	if err != nil || len(keys) != 1 || keys[0].Label != "Laptop" || keys[0].Type != "ssh-ed25519" || keys[0].CreatedAt != 1_700_000_000 {
		t.Fatalf("SSHDeviceKeys() = %#v, %v", keys, err)
	}
	members, err := client.Members(context.Background(), "project-1")
	if err != nil || len(members) != 1 || members[0].Username != "alice" {
		t.Fatalf("Members() = %#v, %v", members, err)
	}

	project, err := client.CreateProject(context.Background(), CreateProjectRequest{Slug: "demo", Name: "Demo", Profile: "go", Source: ProjectSource{Kind: "git", RemoteURL: "ssh://git@example/demo.git"}, InitialPersonIDs: []string{"person-1"}})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if service.projectRequest.GetSource().GetGit().GetRemoteUrl() != "ssh://git@example/demo.git" || len(service.projectRequest.GetInitialPersonIds()) != 1 || project.Profile != "go" || project.Source.Kind != "git" {
		t.Fatalf("project mapping failed: request=%#v result=%#v", service.projectRequest, project)
	}

	host, err := client.HostStatus(context.Background())
	if err != nil {
		t.Fatalf("HostStatus() error = %v", err)
	}
	if host.Overall != "ready" || host.CPUPercent == nil || *host.CPUPercent != 24.5 || host.LoadAverage != [3]float64{1, 2, 3} {
		t.Fatalf("HostStatus() = %#v", host)
	}
}

func TestClientPreservesMissingToolchainAndExplicitRefresh(t *testing.T) {
	client, service := bufconnClient(t)
	installation, err := client.Toolchain(context.Background(), "project-1")
	if err != nil || installation != nil {
		t.Fatalf("Toolchain() = %#v, %v; want nil, nil", installation, err)
	}

	context, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := client.Events(context, "project-1")
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	if event := <-events; event.Kind != "refresh" {
		t.Fatalf("first event = %#v", event)
	}
	if event := <-events; event.Kind != "git_changed" || event.ProjectID == nil || *event.ProjectID != "project-1" || event.Sequence != 9 {
		t.Fatalf("second event = %#v", event)
	}
	if service.eventProjectID == nil || *service.eventProjectID != "project-1" {
		t.Fatalf("SubscribeEvents project ID = %#v", service.eventProjectID)
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
			rpc, ok := err.(RPCError)
			if !ok || rpc.Code != test.code || err.Error() != test.message {
				t.Fatalf("People() error = %#v, want code %s and %q", err, test.code, test.message)
			}
			if test.message != test.detail && strings.Contains(err.Error(), test.detail) {
				t.Fatalf("People() exposed daemon detail %q", test.detail)
			}
		})
	}
}

func TestInvalidStreamControlClosesEventsForRecovery(t *testing.T) {
	client, service := bufconnClient(t)
	service.invalidControl = true
	events, err := client.Events(context.Background(), "")
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	select {
	case event, open := <-events:
		if open {
			t.Fatalf("Events() returned invalid control as %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("Events() did not close after invalid control")
	}
}

func TestNewClientDoesNotWaitForSocketAndRecoversLater(t *testing.T) {
	tempDir, err := os.MkdirTemp("/tmp", "soda-cockpit-grpc-")
	if err != nil {
		t.Fatalf("create socket directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
	socketPath := filepath.Join(tempDir, "sodad.sock")
	started := time.Now()
	client, err := NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("NewClient() blocked for %s", elapsed)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	_, err = client.People(ctx)
	cancel()
	if err == nil || err.Error() != "Soda service unavailable." {
		t.Fatalf("People() before daemon = %v", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen on recovered socket: %v", err)
	}
	server := grpc.NewServer()
	sodav2.RegisterSodaServiceServer(server, &fakeSodaService{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })

	deadline := time.Now().Add(5 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		people, callErr := client.People(ctx)
		cancel()
		if callErr == nil {
			if len(people) != 1 || people[0].Username != "alice" {
				t.Fatalf("People() after recovery = %#v", people)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("People() did not recover: %v", callErr)
		}
		time.Sleep(25 * time.Millisecond)
	}
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
