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

type invalidError struct{ message string }

func (e invalidError) Error() string           { return e.message }
func invalid(format string, args ...any) error { return invalidError{fmt.Sprintf(format, args...)} }

type preconditionError struct{ message string }

func (e preconditionError) Error() string { return e.message }
func precondition(format string, args ...any) error {
	return preconditionError{fmt.Sprintf(format, args...)}
}

func rpcError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok && status.Code(err) != codes.Unknown {
		return err
	}
	var invalidValue invalidError
	if errors.As(err, &invalidValue) {
		return status.Error(codes.InvalidArgument, invalidValue.Error())
	}
	var failed preconditionError
	if errors.As(err, &failed) {
		return status.Error(codes.FailedPrecondition, failed.Error())
	}
	switch {
	case errors.Is(err, store.ErrNotFound), errors.Is(err, host.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, store.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	default:
		return status.Error(codes.Internal, "internal Soda service error")
	}
}
