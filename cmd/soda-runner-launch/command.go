package main

import (
	"errors"
	"strings"

	"github.com/LevitateOS/soda-os/internal/runners"
)

var errUsage = errors.New("usage: soda-runner-launch <runner-id>")

func execute(args, environment []string, launch func(string) (runners.LaunchCommand, error), replace func(runners.LaunchCommand, []string) error) error {
	if len(args) != 1 {
		return errUsage
	}
	command, err := launch(args[0])
	if err != nil {
		return err
	}
	return replace(command, runnerEnvironment(environment, command.Home))
}

func runnerEnvironment(inherited []string, home string) []string {
	// syscall.Exec, unlike os/exec.Cmd, does not deduplicate environment entries.
	environment := make([]string, 0, len(inherited)+1)
	for _, entry := range inherited {
		if !strings.HasPrefix(entry, "HOME=") {
			environment = append(environment, entry)
		}
	}
	return append(environment, "HOME="+home)
}
