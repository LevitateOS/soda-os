package testutil

import (
	"net"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
)

type UnixGRPCServer struct {
	Server     *grpc.Server
	SocketPath string
}

func StartUnixGRPCServer(t testing.TB, register func(*grpc.Server)) *UnixGRPCServer {
	t.Helper()

	socketPath := filepath.Join(t.TempDir(), "sodad.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen on test Unix socket: %v", err)
	}
	server := grpc.NewServer()
	register(server)
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil {
			t.Logf("test gRPC server stopped: %v", serveErr)
		}
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return &UnixGRPCServer{Server: server, SocketPath: socketPath}
}
