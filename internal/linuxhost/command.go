// Package linuxhost exposes the native Linux facts and ordinary account
// operations consumed by Soda's product-specific packages.
package linuxhost

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
)

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Command struct {
	Directory   string
	Name        string
	Args        []string
	Input       io.Reader
	ExtraFiles  []*os.File
	Environment []string
}

type CommandRunner interface {
	Run(context.Context, Command) (CommandResult, error)
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, request Command) (CommandResult, error) {
	command := exec.CommandContext(ctx, request.Name, request.Args...)
	command.Dir = request.Directory
	command.Stdin = request.Input
	command.ExtraFiles = request.ExtraFiles
	command.Env = request.Environment
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	result := CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
		return result, nil
	}
	return result, err
}
