package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: soda-projects <list|add-existing|edit|setup|remove-workspace|remove|delete-human>")
		os.Exit(2)
	}
	current, err := user.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve current Linux account:", err)
		os.Exit(1)
	}
	host := linuxhost.NewNative()
	coordinator := projects.NewSystemCoordinator(host)
	response, err := coordinator.Execute(context.Background(), current.Username, os.Args[1], os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err = encoder.Encode(response); err != nil {
		fmt.Fprintln(os.Stderr, "encode result:", err)
		os.Exit(1)
	}
}
