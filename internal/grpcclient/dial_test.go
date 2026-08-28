package grpcclient

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/version"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type healthServer struct {
	sodav2.UnimplementedSodaServiceServer
}

func (healthServer) Health(context.Context, *sodav2.HealthRequest) (*sodav2.HealthResponse, error) {
	return &sodav2.HealthResponse{Status: "ok", Service: "sodad", Version: version.Version}, nil
}

func TestDialUnixSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "sodad.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	server := grpc.NewServer()
	sodav2.RegisterSodaServiceServer(server, healthServer{})
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil {
			t.Logf("test gRPC server stopped: %v", serveErr)
		}
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connection, err := Dial(ctx, socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })

	response, err := sodav2.NewSodaServiceClient(connection).Health(ctx, &sodav2.HealthRequest{})
	require.NoError(t, err)
	require.Equal(t, version.DefaultVersion, response.Version)
}
