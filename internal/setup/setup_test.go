package setup

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

type fakeAccounts struct{ administrators []Administrator }

func (accounts fakeAccounts) Administrators(context.Context) ([]Administrator, error) {
	return accounts.administrators, nil
}

type fakeNetwork struct {
	connections []Connection
	tailscale   bool
	calls       *[]string
}

func (network *fakeNetwork) Status(context.Context) ([]Connection, bool, error) {
	return append([]Connection(nil), network.connections...), network.tailscale, nil
}

func (network *fakeNetwork) AllowLocalNetwork(_ context.Context, connection string) error {
	*network.calls = append(*network.calls, "network.lan:"+connection)
	for index := range network.connections {
		if network.connections[index].Name == connection {
			network.connections[index].LocalNetworkAllowed = true
			return nil
		}
	}
	return errors.New("connection is not active")
}

func (network *fakeNetwork) ConnectTailscale(_ context.Context, authKey string) error {
	*network.calls = append(*network.calls, "network.tailscale:"+authKey)
	network.tailscale = true
	return nil
}

func testService(connections []Connection, tailscale bool) Service {
	calls := &[]string{}
	return Service{
		Accounts: fakeAccounts{[]Administrator{{Username: "ada"}}},
		Network:  &fakeNetwork{connections: connections, tailscale: tailscale, calls: calls},
	}
}

func TestReadyUsesNativeNetworkWithoutPasswordKeysOrForgejo(t *testing.T) {
	for _, test := range []struct {
		name             string
		connections      []Connection
		tailscale, ready bool
	}{
		{"missing", nil, false, false},
		{"trusted LAN", []Connection{{Name: "wired", LocalNetworkAllowed: true}}, false, true},
		{"Tailscale", nil, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := testService(test.connections, test.tailscale)
			status, err := service.Status(context.Background())
			require.NoError(t, err)
			require.Equal(t, test.ready, status.Ready)
			require.Equal(t, []Administrator{{Username: "ada"}}, status.Administrators)
		})
	}
}

func TestLANAndTailscaleRemainIndependent(t *testing.T) {
	service := testService([]Connection{{Name: "wired"}}, false)
	status, err := service.AllowLocalNetwork(context.Background(), "wired")
	require.NoError(t, err)
	require.True(t, status.Ready)
	require.False(t, status.TailscaleConnected)
	status, err = service.ConnectTailscale(context.Background(), "tskey-auth-secret")
	require.NoError(t, err)
	require.True(t, status.LocalNetworkAllowed)
	require.True(t, status.TailscaleConnected)
}
