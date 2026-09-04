package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"

	"github.com/LevitateOS/soda-os/internal/projects"
)

func main() {
	if projects.IsCredentialHelperInvocation() {
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Git credential operation is required")
			os.Exit(1)
		}
		if err := projects.RunCredentialHelper(os.Args[1], os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: soda-projects <list|add-existing|create-forgejo|edit|setup|remove|delete-human|add-person>")
		os.Exit(2)
	}
	current, err := user.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve current Linux account:", err)
		os.Exit(1)
	}
	catalog := projects.NewCatalog()
	platform := projects.NewNativePlatform()
	lifecycle := projects.Lifecycle{Catalog: catalog, Platform: platform}
	coordinator := projects.Coordinator{
		Catalog: catalog, Lifecycle: lifecycle, Platform: platform,
		Privileged: projects.PKExecInvoker{}, Forgejo: projects.ForgejoClient{},
		Cloner: projects.GitCloner{}, Endpoints: projects.TailnetEndpoints{},
	}
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
