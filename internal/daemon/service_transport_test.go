package daemon

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestHealthRPCOverBufconn(t *testing.T) {
	service := New()
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
	server, err := ListenUnix(socket, New(), slog.Default())
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
