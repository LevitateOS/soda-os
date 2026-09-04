package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/runners"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: soda-runner-launch <runner-id>")
		os.Exit(2)
	}
	command, err := runners.NewNative().Launch(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = os.Chdir(command.Directory); err != nil {
		fmt.Fprintln(os.Stderr, "enter runner state:", err)
		os.Exit(1)
	}
	environment := append(os.Environ(), "HOME="+command.Home)
	if err = syscall.Exec(command.Path, command.Arguments, environment); err != nil {
		fmt.Fprintln(os.Stderr, "start provider runner:", err)
		os.Exit(1)
	}
}
