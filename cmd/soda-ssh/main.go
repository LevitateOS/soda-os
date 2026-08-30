package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/LevitateOS/soda-os/internal/sshgateway"
)

func main() {
	account, err := user.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve Soda person:", err)
		os.Exit(1)
	}

	err = sshgateway.Run(sshgateway.Options{
		Actor:           account.Username,
		Project:         os.Getenv("SODA_PROJECT"),
		Home:            account.HomeDir,
		ProjectsRoot:    os.Getenv("SODA_PROJECTS_ROOT"),
		OriginalCommand: os.Getenv("SSH_ORIGINAL_COMMAND"),
		Shell:           os.Getenv("SHELL"),
		Environment:     os.Environ(),
	}, sshgateway.UnixExecutor{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
