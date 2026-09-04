package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/LevitateOS/soda-os/internal/projects"
	"github.com/LevitateOS/soda-os/internal/runners"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: soda-runner-helper <create|start|stop|restart|remove>")
		os.Exit(2)
	}
	actor, err := projects.PKExecCaller()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	platform := projects.NewNativePlatform()
	authorizer := runners.LinuxAuthorizer{Lifecycle: projects.Lifecycle{Catalog: projects.NewCatalog(), Platform: platform}}
	helper := runners.Helper{Authorizer: authorizer, Lifecycle: runners.NewNative()}
	response, err := helper.Execute(context.Background(), actor.Username, os.Args[1], os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fmt.Fprintln(os.Stderr, "encode result:", err)
		os.Exit(1)
	}
}
