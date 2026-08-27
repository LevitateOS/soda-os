package image

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Command describes one external process invocation. It keeps process execution
// injectable, so the image contract can be tested without Docker or a registry.
type Command struct {
	Dir        string
	Env        []string
	Name       string
	Args       []string
	Privileged bool
}

func (c Command) String() string {
	return strings.TrimSpace(strings.Join(append([]string{c.Name}, c.Args...), " "))
}

// Runner executes external commands for the image builder.
type Runner interface {
	Run(context.Context, Command) error
	Output(context.Context, Command) (string, error)
	CombinedOutput(context.Context, Command) (string, error)
}

// OSRunner is the production process runner.
type OSRunner struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (r OSRunner) Run(ctx context.Context, command Command) error {
	fmt.Fprintf(r.stdout(), "+ %s\n", command.String())
	cmd := r.command(ctx, command)
	cmd.Stdout = r.stdout()
	cmd.Stderr = r.stderr()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", command.String(), err)
	}
	return nil
}

func (r OSRunner) Output(ctx context.Context, command Command) (string, error) {
	fmt.Fprintf(r.stdout(), "+ %s\n", command.String())
	output, err := r.command(ctx, command).Output()
	if err != nil {
		return "", commandError(command, err)
	}
	return string(output), nil
}

func (r OSRunner) CombinedOutput(ctx context.Context, command Command) (string, error) {
	fmt.Fprintf(r.stdout(), "+ %s\n", command.String())
	output, err := r.command(ctx, command).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", command.String(), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func (r OSRunner) command(ctx context.Context, command Command) *exec.Cmd {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir
	if len(command.Env) > 0 {
		cmd.Env = append(cmd.Environ(), command.Env...)
	}
	return cmd
}

func (r OSRunner) stdout() io.Writer {
	if r.Stdout != nil {
		return r.Stdout
	}
	return io.Discard
}

func (r OSRunner) stderr() io.Writer {
	if r.Stderr != nil {
		return r.Stderr
	}
	return io.Discard
}

func commandError(command Command, err error) error {
	var exitError *exec.ExitError
	if !strings.Contains(err.Error(), "exit status") || !errorAs(err, &exitError) {
		return fmt.Errorf("%s: %w", command.String(), err)
	}
	return fmt.Errorf("%s: %w: %s", command.String(), err, strings.TrimSpace(string(exitError.Stderr)))
}

// errorAs is kept separate so tests can provide simple Runner errors without
// relying on an exec.ExitError implementation.
func errorAs(err error, target any) bool {
	return errorsAs(err, target)
}

var errorsAs = func(err error, target any) bool {
	return errors.As(err, target)
}

// RecordingRunner is intentionally small and is useful to command tests.
type RecordingRunner struct {
	Commands []Command
	Outputs  map[string]string
	Err      error
}

func (r *RecordingRunner) Run(_ context.Context, command Command) error {
	r.Commands = append(r.Commands, command)
	return r.Err
}

func (r *RecordingRunner) Output(_ context.Context, command Command) (string, error) {
	r.Commands = append(r.Commands, command)
	if r.Err != nil {
		return "", r.Err
	}
	return r.Outputs[command.String()], nil
}

func (r *RecordingRunner) CombinedOutput(ctx context.Context, command Command) (string, error) {
	return r.Output(ctx, command)
}

func outputText(stdout, stderr string) string {
	var buffer bytes.Buffer
	buffer.WriteString(stdout)
	buffer.WriteString(stderr)
	return buffer.String()
}
