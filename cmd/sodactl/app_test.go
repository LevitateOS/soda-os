package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type recordingServer struct {
	sodav2.UnimplementedSodaServiceServer
	mu  sync.Mutex
	got any
	err error
}

func (s *recordingServer) record(request any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = request
	return s.err
}

func (s *recordingServer) Health(_ context.Context, request *sodav2.HealthRequest) (*sodav2.HealthResponse, error) {
	return &sodav2.HealthResponse{Status: "ready", Service: "sodad", Version: "0.4.0"}, s.record(request)
}

func testApp(t *testing.T, server *recordingServer) (*app, *string) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	sodav2.RegisterSodaServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() { grpcServer.Stop() })
	var socket string
	application := newApp()
	application.dial = func(ctx context.Context, path string) (sodav2.SodaServiceClient, io.Closer, error) {
		socket = path
		conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, nil, err
		}
		return sodav2.NewSodaServiceClient(conn), conn, nil
	}
	return application, &socket
}

func execute(t *testing.T, application *app, args ...string) (string, error) {
	t.Helper()
	root := application.command()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)
	err := root.Execute()
	return output.String(), err
}

func TestHealthCommandUsesResidualGRPCContract(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, any)
	}{
		{"health", []string{"--socket", "/tmp/soda.sock", "health"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.HealthRequest{}, got) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &recordingServer{}
			application, socket := testApp(t, server)
			output, err := execute(t, application, test.args...)
			require.NoError(t, err)
			require.Contains(t, output, "\n")
			test.check(t, server.got)
			require.Equal(t, "/tmp/soda.sock", *socket)
		})
	}
}

func TestRemovedControlPlaneCommandsAreUnavailable(t *testing.T) {
	application := newApp()
	for _, command := range []string{"people", "projects", "os"} {
		_, err := execute(t, application, command)
		require.ErrorContains(t, err, "unknown command")
	}
}

func TestCanonicalGRPCErrorsDoNotExposeInternalDetails(t *testing.T) {
	for _, code := range []codes.Code{codes.Internal, codes.Unknown} {
		t.Run(code.String(), func(t *testing.T) {
			server := &recordingServer{err: status.Error(code, "SQL failure at /var/lib/soda/soda.db: secret")}
			application, _ := testApp(t, server)
			_, err := execute(t, application, "health")
			require.EqualError(t, err, "sodad error: internal service error")
			require.NotContains(t, err.Error(), "SQL")
			require.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestMissingSocketReturnsUnavailableWithinDeadline(t *testing.T) {
	application := newApp()
	application.timeout = 100 * time.Millisecond
	socket := filepath.Join(t.TempDir(), "missing.sock")
	started := time.Now()
	_, err := execute(t, application, "--socket", socket, "health")
	require.EqualError(t, err, "sodad unavailable: Soda service is unavailable")
	require.Less(t, time.Since(started), time.Second)
}

func TestRetainedHealthJSONShape(t *testing.T) {
	application, _ := testApp(t, &recordingServer{})
	output, err := execute(t, application, "health")
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"ready","service":"sodad","version":"0.4.0"}`, output)
}
