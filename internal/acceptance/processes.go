package acceptance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type ProcessSpec struct {
	Name   string
	Args   []string
	Dir    string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

type Process struct {
	command *exec.Cmd
	done    chan error
	once    sync.Once
}

func StartProcess(ctx context.Context, spec ProcessSpec) (*Process, error) {
	command := exec.CommandContext(ctx, spec.Name, spec.Args...)
	command.Dir = spec.Dir
	command.Env = append(command.Environ(), spec.Env...)
	command.Stdout = spec.Stdout
	command.Stderr = spec.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", spec.Name, err)
	}
	process := &Process{command: command, done: make(chan error, 1)}
	go func() { process.done <- command.Wait() }()
	return process, nil
}

func (process *Process) Wait(ctx context.Context) error {
	select {
	case err := <-process.done:
		return process.waitError(err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (process *Process) Stop(ctx context.Context) error {
	var stopErr error
	process.once.Do(func() { stopErr = process.stop(ctx) })
	return stopErr
}

func (process *Process) stop(ctx context.Context) error {
	if process.command.Process == nil {
		return nil
	}
	if err := syscall.Kill(-process.command.Process.Pid, syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate process group %d: %w", process.command.Process.Pid, err)
	}
	select {
	case err := <-process.done:
		return process.stopWaitError(err)
	case <-ctx.Done():
		_ = syscall.Kill(-process.command.Process.Pid, syscall.SIGKILL)
		return fmt.Errorf("stop process group %d: %w", process.command.Process.Pid, ctx.Err())
	}
}

func (process *Process) waitError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("process %d exited: %w", process.command.Process.Pid, err)
}

func (process *Process) stopWaitError(err error) error {
	if err == nil {
		return nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return nil
	}
	return process.waitError(err)
}

func StopDeadline() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
