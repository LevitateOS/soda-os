package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func invalid(format string, args ...any) error {
	return status.Error(codes.InvalidArgument, fmt.Sprintf(format, args...))
}

func precondition(format string, args ...any) error {
	return status.Error(codes.FailedPrecondition, fmt.Sprintf(format, args...))
}

func rpcError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok && status.Code(err) != codes.Unknown {
		return err
	}
	switch {
	case errors.Is(err, store.ErrNotFound), errors.Is(err, host.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, store.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, store.ErrFailedPrecondition):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	default:
		return status.Error(codes.Internal, "internal Soda service error")
	}
}
