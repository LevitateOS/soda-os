package grpcclient

import (
	"context"
	"testing"
	"time"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/testutil"
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
	server := testutil.StartUnixGRPCServer(t, func(server *grpc.Server) {
		sodav2.RegisterSodaServiceServer(server, healthServer{})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connection, err := Dial(ctx, server.SocketPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })

	response, err := sodav2.NewSodaServiceClient(connection).Health(ctx, &sodav2.HealthRequest{})
	require.NoError(t, err)
	require.Equal(t, "0.2.0", response.Version)
}
