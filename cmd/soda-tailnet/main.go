package main

import (
	"context"
	"fmt"
	"time"

	"github.com/LevitateOS/soda-os/internal/tailnet"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status, err := tailnet.New(tailnet.Options{}).Status(ctx)
	fmt.Print(enrollmentMessage(status, err))
}

func enrollmentMessage(status tailnet.Status, statusErr error) string {
	guidance := "Administrator: open Cockpit → Tailscale to connect this machine.\n"
	if statusErr != nil {
		return "\nTailscale status is unavailable.\n" + guidance
	}
	if status.Expired {
		return "\nTailscale authentication has expired.\n" + guidance
	}
	if status.BackendState != "Running" {
		return "\nTailscale: " + connectionDescription(status) + ".\n" + guidance
	}
	message := "\nTailscale: connected.\n"
	if status.Identity != "" {
		message += fmt.Sprintf("Tailnet identity: %s\n", status.Identity)
	}
	if host := status.URLHost(); host != "" {
		return message + fmt.Sprintf("  Cockpit: https://%s:9090\n  Forgejo: http://%s:30000/\n", host, host)
	}
	return message + "Tailnet service addresses are unavailable.\n"
}

func connectionDescription(status tailnet.Status) string {
	if status.BackendState == "NeedsLogin" && status.AuthPending {
		return "waiting for browser authentication"
	}
	descriptions := map[string]string{
		"NeedsLogin": "not signed in", "NeedsMachineAuth": "waiting for Tailnet administrator approval",
		"Stopped": "disconnected", "Starting": "connecting", "NoState": "starting",
	}
	if description, ok := descriptions[status.BackendState]; ok {
		return description
	}
	return status.BackendState
}
