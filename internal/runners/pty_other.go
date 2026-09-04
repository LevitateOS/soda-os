//go:build !linux

package runners

import (
	"context"
	"errors"
)

func (ExecCommandRunner) RunSecret(context.Context, Command, string) error {
	return errors.New("runner registration is supported only on Linux")
}
