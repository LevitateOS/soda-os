package acceptance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

type guestTailnetStatus struct {
	BackendState string `json:"BackendState"`
	Self         struct {
		ID           string   `json:"ID"`
		TailscaleIPs []string `json:"TailscaleIPs"`
	} `json:"Self"`
}

// The guest is already reachable through its known local SSH connection. Read
// its own identity, not the client's peer inventory or a guessed hostname.
func awaitGuestEnrollment(ctx context.Context, installed *guest, admin personFixture) (string, []byte, error) {
	// Enrollment can complete between polls or just as cancellation arrives.
	// The known local connection owns logout from the beginning of this wait.
	installed.enrollment = &guestEnrollment{remote: admin.Remote, password: admin.LinuxPassword}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		raw, err := admin.Remote.Output(ctx, nil, "tailscale", "status", "--json")
		if err != nil {
			return "", nil, fmt.Errorf("read guest enrollment: %w", err)
		}
		var status guestTailnetStatus
		if err = json.Unmarshal(raw, &status); err != nil {
			return "", nil, fmt.Errorf("decode guest enrollment: %w", err)
		}
		if status.BackendState == "Running" {
			host, addressErr := enrolledAddress(status)
			evidence, encodeErr := json.Marshal(status)
			return host, evidence, errors.Join(addressErr, encodeErr)
		}
		select {
		case <-ctx.Done():
			return "", nil, fmt.Errorf("wait for native guest enrollment: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func enrolledAddress(status guestTailnetStatus) (string, error) {
	if status.Self.ID == "" {
		return "", errors.New("enrolled guest returned no native identity")
	}
	for _, address := range status.Self.TailscaleIPs {
		if net.ParseIP(address) != nil {
			return address, nil
		}
	}
	return "", errors.New("enrolled guest returned no usable Tailnet address")
}
