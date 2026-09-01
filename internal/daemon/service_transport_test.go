package daemon

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"github.com/LevitateOS/soda-os/internal/telemetry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type observeHost struct{}

func (observeHost) SampleHost(context.Context) (domain.HostStatus, error) {
	cpu := 12.5
	return domain.HostStatus{
		SampledAt: time.Unix(1, 0),
		Health: domain.HostHealth{
			Overall:  domain.RuntimeReady,
			Services: []domain.ServiceStatus{{Name: "sshd", State: domain.RuntimeReady}},
		},
		Network:  domain.HostNetwork{Interfaces: []domain.NetworkInterface{{Name: "tailscale0", Addresses: []string{"100.64.0.1"}}}},
		Firewall: domain.FirewallStatus{SSHReady: true, CockpitReady: true},
		Resources: domain.HostResources{
			CPUPercent:           &cpu,
			LoadAverage:          [3]float64{1, 2, 3},
			UptimeSeconds:        10,
			MemoryTotalBytes:     20,
			MemoryAvailableBytes: 15,
			Filesystems:          []domain.FilesystemStatus{{Path: "/", TotalBytes: 30, AvailableBytes: 25}},
		},
	}, nil
}

func TestHealthAndHostStatusRPCsOverBufconn(t *testing.T) {
	manager, err := telemetry.NewManager(observeHost{})
	if err != nil {
		t.Fatal(err)
	}
	manager.RefreshHost(context.Background())
	service := New(Options{Telemetry: NewTelemetryAdapter(manager)})
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	sodav2.RegisterSodaServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	connection, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client := sodav2.NewSodaServiceClient(connection)

	health, err := client.Health(context.Background(), &sodav2.HealthRequest{})
	if err != nil || health.GetService() != "sodad" {
		t.Fatalf("health = %#v, %v", health, err)
	}
	host, err := client.GetHostStatus(context.Background(), &sodav2.GetHostStatusRequest{})
	if err != nil || host.GetHost().GetOverall() != sodav2.RuntimeState_RUNTIME_STATE_READY {
		t.Fatalf("host = %#v, %v", host, err)
	}
}

func TestRealUnixSocketPermissionsAndHealth(t *testing.T) {
	if _, err := user.LookupGroup(apiGroup); err != nil {
		t.Skipf("%s group is unavailable: %v", apiGroup, err)
	}
	socketDir, err := os.MkdirTemp("/tmp", "soda-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "s.sock")
	server, err := ListenUnix(socket, New(Options{}), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	t.Cleanup(server.Stop)
	connection, err := grpcclient.Dial(context.Background(), socket)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	health, err := sodav2.NewSodaServiceClient(connection).Health(context.Background(), &sodav2.HealthRequest{})
	if err != nil || health.GetService() != "sodad" {
		t.Fatalf("health = %#v, %v", health, err)
	}
	info, err := os.Stat(socket)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o660 {
		t.Fatalf("socket mode = %o", info.Mode().Perm())
	}
}

func TestUnavailableRuntimeDependenciesUseCanonicalStatus(t *testing.T) {
	service := New(Options{})
	if _, err := service.GetHostStatus(context.Background(), &sodav2.GetHostStatusRequest{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("host status = %s: %v", status.Code(err), err)
	}
}
