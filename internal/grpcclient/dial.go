package grpcclient

import (
	"context"
	"net"

	"github.com/LevitateOS/soda-os/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Dial(ctx context.Context, socketPath string, options ...grpc.DialOption) (*grpc.ClientConn, error) {
	if socketPath == "" {
		socketPath = config.DefaultDaemonSocket
	}
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	}
	defaults := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
	}
	return grpc.DialContext(ctx, "passthrough:///sodad", append(defaults, options...)...)
}
