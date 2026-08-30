package main

import (
	"context"
	"fmt"

	"github.com/LevitateOS/soda-os/internal/tailnet"
)

func main() {
	status, err := tailnet.New(tailnet.Options{}).Status(context.Background())
	fmt.Print(enrollmentMessage(status, err))
}

func enrollmentMessage(status tailnet.Status, statusErr error) string {
	if statusErr != nil {
		return "\nTailscale status is unavailable. Check: sudo systemctl status tailscaled\n"
	}
	switch status.EnrollmentState() {
	case tailnet.Enrolled:
		return fmt.Sprintf("\nTailscale is connected.\nMagicDNS identity: %s\nOpen the Soda OS dashboard:\n  https://%s:9090\n", status.Identity, status.Identity)
	case tailnet.IdentityUnavailable:
		return "\nTailscale is running but has no MagicDNS identity. Tailnet access is unavailable.\n"
	default:
		return "\nTailscale is not enrolled. Tailnet access is unavailable.\nInfrastructure owner: run `sudo tailscale up`, then open the one-time URL it prints to authorize this appliance. After authorization, run `sudo systemctl restart soda-cockpit forgejo` to load the Tailnet dashboard certificate and Forgejo address. Soda does not store a Tailnet authorization key.\n"
	}
}
