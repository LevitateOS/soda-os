package daemon

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"github.com/LevitateOS/soda-os/internal/osupdate"
	"github.com/LevitateOS/soda-os/internal/telemetry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeOSUpdater struct {
	status          osupdate.Status
	candidate       osupdate.Candidate
	stagedReference string
	activated       bool
	err             error
}

func (u *fakeOSUpdater) Status(context.Context) (osupdate.Status, error)   { return u.status, u.err }
func (u *fakeOSUpdater) Check(context.Context) (osupdate.Candidate, error) { return u.candidate, u.err }
func (u *fakeOSUpdater) Stage(_ context.Context, reference string) (osupdate.Status, error) {
	u.stagedReference = reference
	return u.status, u.err
}
func (u *fakeOSUpdater) Activate(context.Context) error {
	u.activated = true
	return u.err
}

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

func TestTemporaryRuntimeRPCsOverBufconn(t *testing.T) {
	manager, err := telemetry.NewManager(observeHost{})
	if err != nil {
		t.Fatal(err)
	}
	manager.RefreshHost(context.Background())
	digest := "sha256:" + strings.Repeat("b", 64)
	updates := &fakeOSUpdater{candidate: osupdate.Candidate{ImageReference: osupdate.Repository + "@" + digest, Digest: digest, Available: true}}
	service := New(Options{Telemetry: NewTelemetryAdapter(manager), OSUpdates: updates})
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
	checked, err := client.CheckOSUpdate(context.Background(), &sodav2.CheckOSUpdateRequest{})
	if err != nil || checked.GetRelease().GetImageReference() != osupdate.Repository+"@"+digest {
		t.Fatalf("release = %#v, %v", checked, err)
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
	if _, err := service.CheckOSUpdate(context.Background(), &sodav2.CheckOSUpdateRequest{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("update status = %s: %v", status.Code(err), err)
	}
}

func exactOSUpdateService() (*Service, *fakeOSUpdater, string) {
	digest := "sha256:" + strings.Repeat("b", 64)
	exact := osupdate.Repository + "@" + digest
	updates := &fakeOSUpdater{
		status:    osupdate.Status{Booted: &osupdate.Deployment{ImageReference: osupdate.Repository + "@sha256:" + strings.Repeat("a", 64), Architecture: "arm64"}},
		candidate: osupdate.Candidate{ImageReference: exact, Digest: digest, Version: "0.3.0", StateSchema: 3, Available: true},
	}
	return New(Options{OSUpdates: updates}), updates, exact
}

func TestOSUpdateRPCsPreserveExactIdentity(t *testing.T) {
	service, updates, exact := exactOSUpdateService()
	checked, err := service.CheckOSUpdate(context.Background(), &sodav2.CheckOSUpdateRequest{})
	if err != nil || checked.GetRelease().GetImageReference() != exact || checked.GetRelease().GetStateSchema() != 3 {
		t.Fatalf("checked release = %#v, %v", checked, err)
	}
	if _, err = service.StageOSUpdate(context.Background(), &sodav2.StageOSUpdateRequest{ImageReference: exact}); err != nil || updates.stagedReference != exact {
		t.Fatalf("staged reference = %q, %v", updates.stagedReference, err)
	}
}

func TestOSUpdateActivationRequiresRebootConfirmation(t *testing.T) {
	service, updates, _ := exactOSUpdateService()
	ctx := context.Background()
	_, err := service.ActivateOSUpdate(ctx, &sodav2.ActivateOSUpdateRequest{})
	if status.Code(err) != codes.InvalidArgument || updates.activated {
		t.Fatalf("unconfirmed activation = %v, %v", updates.activated, err)
	}
	if _, err = service.ActivateOSUpdate(ctx, &sodav2.ActivateOSUpdateRequest{ConfirmReboot: true}); err != nil || !updates.activated {
		t.Fatalf("activation confirmation = %v, %v", updates.activated, err)
	}
}

func TestOSUpdateRejectionUsesCanonicalStatus(t *testing.T) {
	service, updates, _ := exactOSUpdateService()
	updates.err = osupdate.ErrRejected
	_, err := service.CheckOSUpdate(context.Background(), &sodav2.CheckOSUpdateRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("rejected release status = %v", status.Code(err))
	}
}
