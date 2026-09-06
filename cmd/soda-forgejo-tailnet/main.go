// soda-forgejo-tailnet prints the enrolled appliance identity for Forgejo.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/tailnet"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := tailnet.New(tailnet.Options{})
	if err := execute(ctx, os.Stdout, client.Endpoint); err != nil {
		fmt.Fprintln(os.Stderr, "soda-forgejo-tailnet:", err)
		return 1
	}
	return 0
}
