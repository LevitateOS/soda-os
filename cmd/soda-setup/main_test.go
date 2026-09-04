package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/setup"
)

type commandAccounts struct{ administrators []setup.Administrator }

func (accounts commandAccounts) Administrators(context.Context) ([]setup.Administrator, error) {
	return accounts.administrators, nil
}

type commandNetwork struct {
	connections []setup.Connection
	tailscale   bool
	key         string
}

type commandCompletion struct{ dismissed bool }

func (completion *commandCompletion) Dismissed() (bool, error) { return completion.dismissed, nil }
func (completion *commandCompletion) Mark() error {
	completion.dismissed = true
	return nil
}

func (network *commandNetwork) Status(context.Context) ([]setup.Connection, bool, error) {
	return network.connections, network.tailscale, nil
}
func (network *commandNetwork) AllowLocalNetwork(_ context.Context, selected string) error {
	for index := range network.connections {
		if network.connections[index].Name == selected {
			network.connections[index].LocalNetworkAllowed = true
			return nil
		}
	}
	return errors.New("unknown connection")
}
func (network *commandNetwork) ConnectTailscale(_ context.Context, key string) error {
	network.key = key
	network.tailscale = true
	return nil
}

func commandService() (setup.Service, *commandNetwork, *commandCompletion) {
	network := &commandNetwork{connections: []setup.Connection{{Name: "wired"}}}
	completion := &commandCompletion{}
	return setup.Service{Accounts: commandAccounts{administrators: []setup.Administrator{{Username: "ada"}}}, Network: network, Completion: completion}, network, completion
}

func TestConnectTailscaleReadsSecretFromStdinOnly(t *testing.T) {
	service, network, _ := commandService()
	var output bytes.Buffer
	const key = "tskey-auth-one-use-secret"
	if err := execute(context.Background(), service, []string{"connect-tailscale"}, strings.NewReader(`{"auth_key":"`+key+`"}`), &output); err != nil {
		t.Fatal(err)
	}
	if network.key != key {
		t.Fatalf("received key = %q", network.key)
	}
	var response setup.Response
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Status.TailscaleConnected || strings.Contains(output.String(), key) {
		t.Fatalf("response = %s", output.String())
	}
}

func TestPendingTracksNativeProvisioningWithoutDismissal(t *testing.T) {
	service, network, completion := commandService()
	if err := execute(context.Background(), service, []string{"pending"}, nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	network.tailscale = true
	if err := execute(context.Background(), service, []string{"pending"}, nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	completion.dismissed = true
	if err := execute(context.Background(), service, []string{"pending"}, nil, &bytes.Buffer{}); !errors.Is(err, errDismissed) {
		t.Fatal(err)
	}
}

func TestDismissIsAnExplicitReadyNetworkAction(t *testing.T) {
	service, network, completion := commandService()
	network.tailscale = true
	var output bytes.Buffer
	if err := execute(context.Background(), service, []string{"dismiss"}, nil, &output); err != nil {
		t.Fatal(err)
	}
	if !completion.dismissed {
		t.Fatal("dismissal was not recorded")
	}
	var response setup.Response
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Status.Dismissed || response.Status.CanDismiss {
		t.Fatalf("response = %+v", response)
	}
}

func TestConsoleReopensAndReturnsToShellOnQuitOrEOF(t *testing.T) {
	for _, input := range []string{"q\n", ""} {
		service, network, _ := commandService()
		network.tailscale = true
		var output bytes.Buffer
		if err := execute(context.Background(), service, []string{"console"}, strings.NewReader(input), &output); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), "Existing Linux administrator: ada") {
			t.Fatal(output.String())
		}
		for _, forbidden := range []string{"Password:", "SSH public key:", "Create", "Forgejo"} {
			if strings.Contains(output.String(), forbidden) {
				t.Fatal(output.String())
			}
		}
	}
}

func TestMutationErrorsRemainStructured(t *testing.T) {
	service, _, _ := commandService()
	var output bytes.Buffer
	err := execute(context.Background(), service, []string{"allow-local-network"}, strings.NewReader(`{"connection":"missing"}`), &output)
	if err != nil {
		t.Fatal(err)
	}
	var response setup.Response
	if err = json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "unknown connection" || response.Status.Ready {
		t.Fatalf("response = %+v", response)
	}
}

func TestRequestDecoderRejectsUnknownAndTrailingInput(t *testing.T) {
	for _, input := range []string{
		`{"connection":"wired","automatic":true}`,
		`{"connection":"wired"} {}`,
	} {
		service, _, _ := commandService()
		if err := execute(context.Background(), service, []string{"allow-local-network"}, strings.NewReader(input), &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted request %s", input)
		}
	}
}
