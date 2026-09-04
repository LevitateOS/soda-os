package projects

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Cloner interface {
	Clone(context.Context, string, string) error
}

type GitCloner struct {
	Binary string
	Stdout io.Writer
	Stderr io.Writer
}

func (cloner GitCloner) Clone(ctx context.Context, remote, destination string) error {
	if err := validateCloneTarget(remote, destination); err != nil {
		return err
	}
	binary := cloner.Binary
	if binary == "" {
		binary = "/usr/bin/git"
	}
	command := cloner.cloneCommand(ctx, binary, remote, destination)
	if err := command.Run(); err != nil {
		return fmt.Errorf("Git clone failed: %w", err)
	}
	return nil
}

func validateCloneTarget(remote, destination string) error {
	if err := ValidateCanonicalURL(remote); err != nil {
		return fmt.Errorf("clone URL: %w", err)
	}
	if destination == "" {
		return errors.New("clone destination is required")
	}
	_, err := os.Lstat(destination)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err == nil {
		return errors.New("clone destination already exists")
	}
	return fmt.Errorf("inspect clone destination: %w", err)
}

func (cloner GitCloner) cloneCommand(ctx context.Context, binary, remote, destination string) *exec.Cmd {
	command := exec.CommandContext(ctx, binary, "clone", "--", remote, destination)
	command.Stdout = cloner.Stdout
	if command.Stdout == nil {
		command.Stdout = os.Stdout
	}
	command.Stderr = cloner.Stderr
	if command.Stderr == nil {
		command.Stderr = os.Stderr
	}
	command.Env = environmentWithOverrides(os.Environ(), map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"LC_ALL":              "C",
	})
	return command
}

func environmentWithOverrides(environment []string, overrides map[string]string) []string {
	result := make([]string, 0, len(environment)+len(overrides))
	for _, value := range environment {
		name, _, _ := strings.Cut(value, "=")
		if _, replaced := overrides[name]; !replaced {
			result = append(result, value)
		}
	}
	for name, value := range overrides {
		result = append(result, name+"="+value)
	}
	return result
}
