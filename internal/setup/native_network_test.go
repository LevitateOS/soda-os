package setup

import (
	"context"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/tailnet"
)

type networkRunner struct {
	requests []linuxhost.Command
	secret   string
}

func (runner *networkRunner) Run(_ context.Context, request linuxhost.Command) (linuxhost.CommandResult, error) {
	runner.requests = append(runner.requests, request)
	if request.Name == "/usr/bin/nmcli" {
		return networkManagerResult(request), nil
	}
	switch request.Name {
	case "/usr/bin/tailscale":
		if len(request.ExtraFiles) != 1 {
			return linuxhost.CommandResult{ExitCode: 1}, nil
		}
		contents, err := io.ReadAll(request.ExtraFiles[0])
		if err != nil {
			return linuxhost.CommandResult{}, err
		}
		runner.secret = string(contents)
	}
	return linuxhost.CommandResult{}, nil
}

func networkManagerResult(request linuxhost.Command) linuxhost.CommandResult {
	if reflect.DeepEqual(request.Args, []string{"--get-values", "NAME", "connection", "show", "--active"}) {
		return linuxhost.CommandResult{Stdout: "wired\nTailscale\nlo\n"}
	}
	if request.Args[1] == "connection.type" && request.Args[len(request.Args)-1] == "lo" {
		return linuxhost.CommandResult{Stdout: "loopback\n"}
	}
	if request.Args[1] == "connection.type" {
		return linuxhost.CommandResult{Stdout: "802-3-ethernet\n"}
	}
	if request.Args[len(request.Args)-1] == "wired" {
		return linuxhost.CommandResult{Stdout: "trusted\n"}
	}
	return linuxhost.CommandResult{}
}

type connectedTailnet struct{}

func (connectedTailnet) Status(context.Context) (tailnet.Status, error) {
	return tailnet.Status{BackendState: "Running", Identity: "soda.example.ts.net"}, nil
}

func TestConnectTailscalePassesKeyOnlyThroughAnonymousDescriptor(t *testing.T) {
	runner := &networkRunner{}
	network := NativeNetwork{Runner: runner, Tailnet: connectedTailnet{}}
	const key = "tskey-auth-one-use-secret"
	if err := network.ConnectTailscale(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if runner.secret != key {
		t.Fatalf("descriptor contents = %q", runner.secret)
	}
	if len(runner.requests) != 2 {
		t.Fatalf("requests = %#v", runner.requests)
	}
	tailscale := runner.requests[0]
	if tailscale.Name != "/usr/bin/tailscale" || !reflect.DeepEqual(tailscale.Args, []string{"up", "--auth-key=file:/proc/self/fd/3"}) {
		t.Fatalf("Tailscale command = %#v", tailscale)
	}
	for _, value := range append(append([]string{}, tailscale.Args...), tailscale.Environment...) {
		if strings.Contains(value, key) {
			t.Fatalf("key exposed in process metadata: %q", value)
		}
	}
}

func TestNetworkStatusReportsIndependentNativeFacts(t *testing.T) {
	runner := &networkRunner{}
	network := NativeNetwork{Runner: runner, Tailnet: connectedTailnet{}}
	connections, tailscaleConnected, err := network.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []Connection{{Name: "Tailscale"}, {Name: "wired", LocalNetworkAllowed: true}}
	if !reflect.DeepEqual(connections, want) || !tailscaleConnected {
		t.Fatalf("Status() = (%#v, %v), want (%#v, true)", connections, tailscaleConnected, want)
	}
}

func TestSealedSecretCannotBeModified(t *testing.T) {
	secret, err := sealedSecret([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	defer secret.Close()
	if _, err = secret.WriteAt([]byte("X"), 0); err == nil {
		t.Fatal("sealed secret remained writable")
	}
}
