package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LevitateOS/soda-os/internal/sshgateway"
)

func main() {
	actor := flag.String("actor", "", "Soda person entering the worktree")
	worktree := flag.String("worktree", "", "Soda project worktree to enter")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "soda-ssh does not accept positional arguments")
		os.Exit(2)
	}

	err := sshgateway.Run(sshgateway.Options{
		Actor:           *actor,
		Worktree:        *worktree,
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
