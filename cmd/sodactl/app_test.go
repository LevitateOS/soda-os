package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"path/filepath"
	"strings"
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

func (s *recordingServer) GetOSUpdateStatus(_ context.Context, request *sodav2.GetOSUpdateStatusRequest) (*sodav2.GetOSUpdateStatusResponse, error) {
	return &sodav2.GetOSUpdateStatusResponse{Status: testOSUpdateStatus()}, s.record(request)
}

func (s *recordingServer) CheckOSUpdate(_ context.Context, request *sodav2.CheckOSUpdateRequest) (*sodav2.CheckOSUpdateResponse, error) {
	return &sodav2.CheckOSUpdateResponse{Release: &sodav2.OSRelease{ImageReference: "ghcr.io/levitateos/soda-os@sha256:" + strings.Repeat("b", 64), Version: "0.5.0", Digest: "sha256:" + strings.Repeat("b", 64), StateSchema: 4, Available: true}}, s.record(request)
}

func (s *recordingServer) StageOSUpdate(_ context.Context, request *sodav2.StageOSUpdateRequest) (*sodav2.StageOSUpdateResponse, error) {
	return &sodav2.StageOSUpdateResponse{Status: testOSUpdateStatus()}, s.record(request)
}

func (s *recordingServer) ActivateOSUpdate(_ context.Context, request *sodav2.ActivateOSUpdateRequest) (*sodav2.ActivateOSUpdateResponse, error) {
	return &sodav2.ActivateOSUpdateResponse{RebootRequested: request.GetConfirmReboot()}, s.record(request)
}

func testOSUpdateStatus() *sodav2.OSUpdateStatus {
	digest := "sha256:" + strings.Repeat("a", 64)
	return &sodav2.OSUpdateStatus{ReadOnly: true, Booted: &sodav2.OSDeployment{ImageReference: "ghcr.io/levitateos/soda-os@" + digest, Version: "0.4.0", Digest: digest, Architecture: "amd64"}}
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

func TestCommandsUseReducedGRPCContract(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, any)
	}{
		{"health", []string{"--socket", "/tmp/soda.sock", "health"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.HealthRequest{}, got) }},
		{"update status", []string{"os", "update", "status"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.GetOSUpdateStatusRequest{}, got) }},
		{"update check", []string{"os", "update", "check"}, func(t *testing.T, got any) { require.IsType(t, &sodav2.CheckOSUpdateRequest{}, got) }},
		{"update stage", []string{"os", "update", "stage"}, func(t *testing.T, got any) {
			request := got.(*sodav2.StageOSUpdateRequest)
			require.Equal(t, "ghcr.io/levitateos/soda-os@sha256:"+strings.Repeat("b", 64), request.GetImageReference())
		}},
		{"update activate", []string{"os", "update", "activate", "--confirm-reboot"}, func(t *testing.T, got any) {
			require.True(t, got.(*sodav2.ActivateOSUpdateRequest).GetConfirmReboot())
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &recordingServer{}
			application, socket := testApp(t, server)
			output, err := execute(t, application, test.args...)
			require.NoError(t, err)
			require.Contains(t, output, "\n")
			test.check(t, server.got)
			if test.name == "health" {
				require.Equal(t, "/tmp/soda.sock", *socket)
			}
		})
	}
}

func TestRemovedControlPlaneCommandsAreUnavailable(t *testing.T) {
	application := newApp()
	for _, command := range []string{"people", "projects"} {
		_, err := execute(t, application, command)
		require.ErrorContains(t, err, "unknown command")
	}
}

func TestOSActivationRequiresExplicitConfirmationBeforeDial(t *testing.T) {
	application := newApp()
	dials := 0
	application.dial = func(context.Context, string) (sodav2.SodaServiceClient, io.Closer, error) {
		dials++
		return nil, nil, io.ErrUnexpectedEOF
	}
	_, err := execute(t, application, "os", "update", "activate")
	require.ErrorContains(t, err, "required flag(s) \"confirm-reboot\" not set")
	require.Zero(t, dials)
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

func TestRetainedJSONShapes(t *testing.T) {
	application, _ := testApp(t, &recordingServer{})
	output, err := execute(t, application, "health")
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"ready","service":"sodad","version":"0.4.0"}`, output)

	application, _ = testApp(t, &recordingServer{})
	output, err = execute(t, application, "os", "update", "status")
	require.NoError(t, err)
	require.JSONEq(t, `{"booted":{"image_reference":"ghcr.io/levitateos/soda-os@sha256:`+strings.Repeat("a", 64)+`","version":"0.4.0","digest":"sha256:`+strings.Repeat("a", 64)+`","architecture":"amd64","incompatible":false,"download_only":false},"staged":null,"read_only":true}`, output)
}
