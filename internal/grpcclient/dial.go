package grpcclient

import (
	"context"
	"net"

	"github.com/LevitateOS/soda-os/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Dial(ctx context.Context, socketPath string, options ...grpc.DialOption) (*grpc.ClientConn, error) {
	return dial(ctx, socketPath, append(options, grpc.WithBlock())...)
}

func New(socketPath string, options ...grpc.DialOption) (*grpc.ClientConn, error) {
	if socketPath == "" {
		socketPath = config.DefaultDaemonSocket
	}
	dialer := unixDialer(socketPath)
	defaults := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	}
	return grpc.NewClient("passthrough:///sodad", append(defaults, options...)...)
}

func dial(ctx context.Context, socketPath string, options ...grpc.DialOption) (*grpc.ClientConn, error) {
	if socketPath == "" {
		socketPath = config.DefaultDaemonSocket
	}
	dialer := unixDialer(socketPath)
	defaults := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	}
	return grpc.DialContext(ctx, "passthrough:///sodad", append(defaults, options...)...)
}

func unixDialer(socketPath string) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	}
}
