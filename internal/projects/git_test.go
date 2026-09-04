package projects

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitCloneInvokesNativeGitForSSHRemote(t *testing.T) {
	root := t.TempDir()
	probe := filepath.Join(root, "git-probe")
	require.NoError(t, os.WriteFile(probe, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" >\"$SODA_TEST_GIT_CAPTURE\"\n"), 0o700))
	capture := filepath.Join(root, "capture")
	t.Setenv("SODA_TEST_GIT_CAPTURE", capture)
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	destination := filepath.Join(root, "checkout")
	remote := "git@git.example.test:team/site.git"

	require.NoError(t, (GitCloner{Binary: probe, Stdout: io.Discard, Stderr: io.Discard}).Clone(context.Background(), remote, destination))
	arguments, err := os.ReadFile(capture)
	require.NoError(t, err)
	require.Equal(t, strings.Join([]string{"clone", "--", remote, destination, ""}, "\n"), string(arguments))
}

func TestGitCloneRejectsNonSSHRemoteBeforeRunningGit(t *testing.T) {
	root := t.TempDir()
	probe := filepath.Join(root, "git-probe")
	require.NoError(t, os.WriteFile(probe, []byte("#!/bin/sh\nexit 99\n"), 0o700))

	err := (GitCloner{Binary: probe, Stdout: io.Discard, Stderr: io.Discard}).Clone(
		context.Background(), "https://git.example.test/team/site.git", filepath.Join(root, "checkout"),
	)
	require.ErrorContains(t, err, "must use SSH or SCP syntax")
}

func TestGitCloneRejectsExistingDestination(t *testing.T) {
	destination := t.TempDir()
	err := (GitCloner{Stdout: io.Discard, Stderr: io.Discard}).Clone(
		context.Background(), "ssh://git@git.example.test/team/site.git", destination,
	)
	require.ErrorContains(t, err, "destination already exists")
}

func TestGitCloneDisablesInteractiveTerminalPrompt(t *testing.T) {
	command := (GitCloner{}).cloneCommand(context.Background(), "/usr/bin/git", "git@example.test:team/site.git", "/tmp/site")
	require.Contains(t, command.Env, "GIT_TERMINAL_PROMPT=0")
}
