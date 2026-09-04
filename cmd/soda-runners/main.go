package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/runners"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: soda-runners <list|create|start|stop|restart|remove>")
		os.Exit(2)
	}
	current, err := user.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve current Linux account:", err)
		os.Exit(1)
	}
	accounts := linuxhost.NewNative()
	authorizer := runners.LinuxAuthorizer{Accounts: accounts}
	coordinator := runners.Coordinator{
		Authorizer: authorizer,
		Local:      runners.NewNative(),
		Privileged: runners.PKExecInvoker{},
	}
	actor := linuxhost.PKExecIdentity{Username: current.Username, UID: os.Getuid()}
	response, err := coordinator.Execute(context.Background(), actor, os.Args[1], os.Stdin)
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
