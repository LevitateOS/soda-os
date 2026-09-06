package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/runners"
)

func main() { os.Exit(run()) }

func run() int {
	// No signal interception: exec must retain ordinary native process behavior.
	if err := execute(os.Args[1:], os.Environ(), runners.NewNative().Launch, replaceProcess); err != nil {
		fmt.Fprintln(os.Stderr, "soda-runner-launch:", err)
		if errors.Is(err, errUsage) {
			return 2
		}
		return 1
	}
	return 0
}

func replaceProcess(command runners.LaunchCommand, environment []string) error {
	if err := os.Chdir(command.Directory); err != nil {
		return fmt.Errorf("enter runner state: %w", err)
	}
	if err := syscall.Exec(command.Path, command.Arguments, environment); err != nil {
		return fmt.Errorf("start provider runner: %w", err)
	}
	return nil
}
