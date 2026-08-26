package soda

import (
	"context"
	"net"
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
}

func (s *fakeSodaService) ListPeople(context.Context, *sodav2.ListPeopleRequest) (*sodav2.ListPeopleResponse, error) {
	return &sodav2.ListPeopleResponse{People: []*sodav2.Person{{Id: "person-1", Username: "alice", DisplayName: "Alice", Email: "alice@soda.local", Role: sodav2.Role_ROLE_ADMIN, SshPublicKey: "ssh-ed25519 AAAA"}}}, nil
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

func (*fakeSodaService) SubscribeEvents(_ *sodav2.SubscribeEventsRequest, stream sodav2.SodaService_SubscribeEventsServer) error {
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

	project, err := client.CreateProject(context.Background(), CreateProjectRequest{Slug: "demo", Name: "Demo", Profile: "go", Source: ProjectSource{Kind: "git", RemoteURL: "ssh://git@example/demo.git"}})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if service.projectRequest.GetSource().GetGit().GetRemoteUrl() != "ssh://git@example/demo.git" || project.Profile != "go" || project.Source.Kind != "git" {
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
	client, _ := bufconnClient(t)
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
}

func TestClientTurnsGRPCErrorsIntoUsefulErrors(t *testing.T) {
	client := newClient(failingService{}, nil)
	_, err := client.People(context.Background())
	if err == nil {
		t.Fatal("People() error = nil")
	}
	rpc, ok := err.(RPCError)
	if !ok || rpc.Code != codes.Unavailable || rpc.Message != "daemon unavailable" {
		t.Fatalf("People() error = %#v", err)
	}
}

type failingService struct{ sodav2.SodaServiceClient }

func (failingService) ListPeople(context.Context, *sodav2.ListPeopleRequest, ...grpc.CallOption) (*sodav2.ListPeopleResponse, error) {
	return nil, status.Error(codes.Unavailable, "daemon unavailable")
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
