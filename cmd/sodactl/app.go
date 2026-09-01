package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/LevitateOS/soda-os/internal/config"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultCommandTimeout = 2 * time.Second
)

type dialFunc func(context.Context, string) (sodav2.SodaServiceClient, io.Closer, error)

type app struct {
	dial    dialFunc
	timeout time.Duration
}

func newApp() *app {
	return &app{
		dial: func(ctx context.Context, socket string) (sodav2.SodaServiceClient, io.Closer, error) {
			conn, err := grpcclient.Dial(ctx, socket)
			if err != nil {
				return nil, nil, err
			}
			return sodav2.NewSodaServiceClient(conn), conn, nil
		},
		timeout: defaultCommandTimeout,
	}
}

func (a *app) command() *cobra.Command {
	var socket string
	root := &cobra.Command{
		Use:          "sodactl",
		Short:        "Administer Soda OS",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&socket, "socket", config.DefaultDaemonSocket, "sodad Unix socket")
	root.AddCommand(a.healthCommand(&socket))
	return root
}

func (a *app) healthCommand(socket *string) *cobra.Command {
	return &cobra.Command{
		Use: "health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.call(cmd, *socket, func(ctx context.Context, client sodav2.SodaServiceClient) (any, error) {
				response, err := client.Health(ctx, &sodav2.HealthRequest{})
				return healthJSON(response), err
			})
		},
	}
}

func (a *app) call(command *cobra.Command, socket string, operation func(context.Context, sodav2.SodaServiceClient) (any, error)) error {
	return a.callWithTimeout(command, socket, a.timeout, operation)
}

func (a *app) callWithTimeout(command *cobra.Command, socket string, timeout time.Duration, operation func(context.Context, sodav2.SodaServiceClient) (any, error)) error {
	ctx, cancel := context.WithTimeout(command.Context(), timeout)
	defer cancel()
	client, closer, err := a.dial(ctx, socket)
	if err != nil {
		return errors.New("sodad unavailable: Soda service is unavailable")
	}
	defer closer.Close()
	response, err := operation(ctx, client)
	if err != nil {
		return canonicalError(err)
	}
	encoded, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	_, err = fmt.Fprintln(command.Root().OutOrStdout(), string(encoded))
	return err
}

func canonicalError(err error) error {
	grpcStatus, ok := status.FromError(err)
	if !ok {
		return err
	}
	prefix := "sodad error"
	switch grpcStatus.Code() {
	case codes.InvalidArgument:
		prefix = "invalid input"
	case codes.NotFound:
		prefix = "not found"
	case codes.AlreadyExists:
		prefix = "conflict"
	case codes.PermissionDenied, codes.Unauthenticated:
		prefix = "permission denied"
	case codes.FailedPrecondition:
		prefix = "operation rejected"
	case codes.Unavailable:
		return errors.New("sodad unavailable: Soda service is unavailable")
	case codes.DeadlineExceeded:
		return errors.New("sodad timed out: Soda service did not respond in time")
	case codes.Internal, codes.Unknown:
		return errors.New("sodad error: internal service error")
	}
	return fmt.Errorf("%s: %s", prefix, grpcStatus.Message())
}
