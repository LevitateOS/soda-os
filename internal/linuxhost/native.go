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
	return native.runner().Run(ctx, command)
}

func (native *Native) run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	return native.Run(ctx, Command{Name: name, Args: args})
}

func (native *Native) runner() CommandRunner {
	if native.Runner == nil {
		return ExecCommandRunner{}
	}
	return native.Runner
}

func (native *Native) homeRoot() string {
	if native.HomeRoot == "" {
		return "/home"
	}
	return native.HomeRoot
}
