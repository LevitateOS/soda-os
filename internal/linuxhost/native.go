package linuxhost

import (
	"context"
	"errors"
)

var ErrAccountNotFound = errors.New("Linux account not found")

type Native struct {
	Runner        CommandRunner
	LoginDefsPath string
	HomeRoot      string
}

func NewNative() *Native {
	return &Native{
		Runner:        ExecCommandRunner{},
		LoginDefsPath: "/etc/login.defs",
		HomeRoot:      "/home",
	}
}

func (native *Native) Run(ctx context.Context, command Command) (CommandResult, error) {
	if native == nil || native.Runner == nil {
		return CommandResult{}, errors.New("Linux host command runner was not constructed")
	}
	return native.Runner.Run(ctx, command)
}

func (native *Native) run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	return native.Run(ctx, Command{Name: name, Args: args})
}

func (native *Native) homeRoot() (string, error) {
	if native == nil || native.HomeRoot == "" {
		return "", errors.New("Linux host home root was not constructed")
	}
	return native.HomeRoot, nil
}
