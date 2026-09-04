package acceptance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"time"
)

type tailnetStatus struct {
	Peer map[string]tailnetPeer `json:"Peer"`
}

type tailnetPeer struct {
	ID           string   `json:"ID"`
	HostName     string   `json:"HostName"`
	Online       bool     `json:"Online"`
	TailscaleIPs []string `json:"TailscaleIPs"`
}

type Tailnet struct {
	Binary string
}

func NewTailnet() (Tailnet, error) {
	if path, err := executablePath("tailscale"); err == nil {
		return Tailnet{Binary: path}, nil
	}
	path := "/Applications/Tailscale.app/Contents/MacOS/Tailscale"
	if runtime.GOOS == "darwin" {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return Tailnet{Binary: path}, nil
		}
	}
	return Tailnet{}, errors.New("Tailscale CLI is unavailable")
}

func (tailnet Tailnet) Snapshot(ctx context.Context) (tailnetStatus, []byte, error) {
	contents, err := CommandOutput(ctx, CommandSpec{Name: tailnet.Binary, Args: []string{"status", "--json"}, Env: []string{"TAILSCALE_BE_CLI=1"}})
	if err != nil {
		return tailnetStatus{}, nil, err
	}
	var status tailnetStatus
	if err = json.Unmarshal(contents, &status); err != nil {
		return tailnetStatus{}, nil, fmt.Errorf("decode Tailscale status: %w", err)
	}
	return status, contents, nil
}

func (tailnet Tailnet) Discover(ctx context.Context, before tailnetStatus) (string, []byte, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		status, raw, err := tailnet.Snapshot(ctx)
		if err == nil {
			address, found, discoveryErr := newSodaPeer(before, status)
			if found || discoveryErr != nil {
				return address, raw, discoveryErr
			}
		}
		select {
		case <-ctx.Done():
			return "", nil, fmt.Errorf("discover enrolled Soda peer: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func newSodaPeer(before, after tailnetStatus) (string, bool, error) {
	addresses := []string{}
	for id, peer := range after.Peer {
		if _, existed := before.Peer[id]; existed || !peer.Online || peer.HostName != "soda" || len(peer.TailscaleIPs) == 0 {
			continue
		}
		addresses = append(addresses, peer.TailscaleIPs[0])
	}
	sort.Strings(addresses)
	if len(addresses) == 1 {
		return addresses[0], true, nil
	}
	if len(addresses) > 1 {
		return "", false, errors.New("multiple newly enrolled Soda peers are online")
	}
	return "", false, nil
}

func executablePath(name string) (string, error) {
	return exec.LookPath(name)
}
