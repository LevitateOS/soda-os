package acceptance

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type CommandSpec struct {
	Name   string
	Args   []string
	Dir    string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func RunCommand(ctx context.Context, spec CommandSpec) error {
	command := externalCommand(ctx, spec)
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s: %w", commandLabel(spec), err)
	}
	return nil
}

func CommandOutput(ctx context.Context, spec CommandSpec) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	spec.Stdout = &stdout
	spec.Stderr = &stderr
	if err := RunCommand(ctx, spec); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return nil, fmt.Errorf("%w: %s", err, message)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

func externalCommand(ctx context.Context, spec CommandSpec) *exec.Cmd {
	command := exec.CommandContext(ctx, spec.Name, spec.Args...)
	command.Dir = spec.Dir
	command.Stdin = spec.Stdin
	command.Stdout = spec.Stdout
	command.Stderr = spec.Stderr
	if len(spec.Env) > 0 {
		command.Env = append(command.Environ(), spec.Env...)
	}
	return command
}

func commandLabel(spec CommandSpec) string {
	return strings.Join(append([]string{spec.Name}, spec.Args...), " ")
}

func RequireCommands(commands ...string) error {
	for _, command := range commands {
		if _, err := exec.LookPath(command); err != nil {
			return fmt.Errorf("required command %s is unavailable", command)
		}
	}
	return nil
}

func newPrivateFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
}
