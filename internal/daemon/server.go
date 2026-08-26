package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/user"
	"runtime/debug"
	"strconv"
	"time"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	grpc     *grpc.Server
	listener net.Listener
	socket   string
}

func ListenUnix(socketPath, group string, service sodav2.SodaServiceServer, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(filepathDir(socketPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen on Soda socket: %w", err)
	}
	cleanup := func() { listener.Close(); os.Remove(socketPath) }
	if err = os.Chmod(socketPath, 0o660); err != nil {
		cleanup()
		return nil, err
	}
	if group != "" {
		entry, lookupErr := user.LookupGroup(group)
		if lookupErr != nil {
			cleanup()
			return nil, fmt.Errorf("look up socket group %q: %w", group, lookupErr)
		}
		gid, parseErr := strconv.Atoi(entry.Gid)
		if parseErr != nil {
			cleanup()
			return nil, parseErr
		}
		if err = os.Chown(socketPath, -1, gid); err != nil {
			cleanup()
			return nil, err
		}
	}
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(recoveryUnary(logger), loggingUnary(logger)),
		grpc.ChainStreamInterceptor(recoveryStream(logger), loggingStream(logger)),
	)
	sodav2.RegisterSodaServiceServer(server, service)
	return &Server{grpc: server, listener: listener, socket: socketPath}, nil
}
func (s *Server) Serve() error { return s.grpc.Serve(s.listener) }
func (s *Server) Stop()        { s.grpc.Stop(); s.listener.Close(); os.Remove(s.socket) }

func recoveryUnary(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic in gRPC method", slog.String("method", info.FullMethod), slog.Any("panic", recovered), slog.String("stack", string(debug.Stack())))
				err = status.Error(codes.Internal, "internal Soda service error")
			}
		}()
		return handler(ctx, request)
	}
}
func loggingUnary(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()
		response, err := handler(ctx, request)
		logger.LogAttrs(ctx, slog.LevelInfo, "gRPC request", slog.String("method", info.FullMethod), slog.Duration("duration", time.Since(started)), slog.String("status", status.Code(err).String()))
		return response, err
	}
}
func recoveryStream(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(service any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic in gRPC stream", slog.String("method", info.FullMethod), slog.Any("panic", recovered), slog.String("stack", string(debug.Stack())))
				err = status.Error(codes.Internal, "internal Soda service error")
			}
		}()
		return handler(service, stream)
	}
}
func loggingStream(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(service any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		started := time.Now()
		err := handler(service, stream)
		logger.LogAttrs(stream.Context(), slog.LevelInfo, "gRPC stream", slog.String("method", info.FullMethod), slog.Duration("duration", time.Since(started)), slog.String("status", status.Code(err).String()))
		return err
	}
}
func filepathDir(path string) string {
	for index := len(path) - 1; index >= 0; index-- {
		if path[index] == '/' {
			if index == 0 {
				return "/"
			}
			return path[:index]
		}
	}
	return "."
}
