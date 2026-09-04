package setup

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeAccounts struct {
	administrators []Administrator
	prepareErr     error
	promoteErr     error
	calls          *[]string
}

func (accounts *fakeAccounts) Administrators(context.Context) ([]Administrator, error) {
	return append([]Administrator(nil), accounts.administrators...), nil
}

func (accounts *fakeAccounts) Prepare(_ context.Context, request AdministratorRequest) error {
	*accounts.calls = append(*accounts.calls, "accounts.prepare:"+request.Username)
	return accounts.prepareErr
}

func (accounts *fakeAccounts) Promote(_ context.Context, username string) error {
	*accounts.calls = append(*accounts.calls, "accounts.promote:"+username)
	if accounts.promoteErr == nil {
		accounts.administrators = []Administrator{{Username: username, PasswordSet: true, SSHPublicKey: true}}
	}
	return accounts.promoteErr
}

type fakeForgejo struct {
	ready      map[string]bool
	prepareErr error
	calls      *[]string
}

func (forgejo *fakeForgejo) Ready(_ context.Context, username string) bool {
	return forgejo.ready[username]
}

func (forgejo *fakeForgejo) PrepareAdministrator(_ context.Context, request AdministratorRequest) error {
	*forgejo.calls = append(*forgejo.calls, "forgejo.prepare:"+request.Username)
	if forgejo.prepareErr == nil {
		forgejo.ready[request.Username] = true
	}
	return forgejo.prepareErr
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

type fakeCompletion struct{ dismissed bool }

func (completion *fakeCompletion) Dismissed() (bool, error) { return completion.dismissed, nil }
func (completion *fakeCompletion) Mark() error {
	completion.dismissed = true
	return nil
}

func testService(administrators []Administrator, forgejoReady bool, connections []Connection, tailscale bool) (Service, *[]string) {
	calls := &[]string{}
	ready := map[string]bool{}
	for _, administrator := range administrators {
		ready[administrator.Username] = forgejoReady
	}
	return Service{
		Accounts:   &fakeAccounts{administrators: administrators, calls: calls},
		Forgejo:    &fakeForgejo{ready: ready, calls: calls},
		Network:    &fakeNetwork{connections: connections, tailscale: tailscale, calls: calls},
		Completion: &fakeCompletion{},
	}, calls
}

func TestDismissRequiresCompleteAdministratorAndApprovedNetwork(t *testing.T) {
	complete := Administrator{Username: "ada", PasswordSet: true, SSHPublicKey: true}
	tests := []struct {
		name           string
		administrators []Administrator
		forgejoReady   bool
		connections    []Connection
		tailscale      bool
		want           bool
	}{
		{name: "no administrator", connections: []Connection{{Name: "wired", LocalNetworkAllowed: true}}},
		{name: "missing password", administrators: []Administrator{{Username: "ada", SSHPublicKey: true}}, forgejoReady: true, tailscale: true},
		{name: "missing key", administrators: []Administrator{{Username: "ada", PasswordSet: true}}, forgejoReady: true, tailscale: true},
		{name: "missing Forgejo", administrators: []Administrator{complete}, tailscale: true},
		{name: "missing network", administrators: []Administrator{complete}, forgejoReady: true},
		{name: "trusted LAN", administrators: []Administrator{complete}, forgejoReady: true, connections: []Connection{{Name: "wired", LocalNetworkAllowed: true}}, want: true},
		{name: "Tailscale", administrators: []Administrator{complete}, forgejoReady: true, tailscale: true, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, _ := testService(test.administrators, test.forgejoReady, test.connections, test.tailscale)
			status, err := service.Status(context.Background())
			require.NoError(t, err)
			require.Equal(t, test.want, status.CanDismiss)
			status, err = service.Dismiss(context.Background())
			if test.want {
				require.NoError(t, err)
				require.True(t, status.Dismissed)
				return
			}
			require.Error(t, err)
			require.False(t, status.Dismissed)
		})
	}
}

func TestLANAndTailscaleRemainIndependent(t *testing.T) {
	service, _ := testService(
		[]Administrator{{Username: "ada", PasswordSet: true, SSHPublicKey: true}}, true,
		[]Connection{{Name: "wired"}}, false,
	)
	status, err := service.AllowLocalNetwork(context.Background(), "wired")
	if err != nil || !status.LocalNetworkAllowed || status.TailscaleConnected {
		t.Fatalf("AllowLocalNetwork() = (%+v, %v)", status, err)
	}
	status, err = service.ConnectTailscale(context.Background(), "tskey-auth-secret")
	if err != nil || !status.LocalNetworkAllowed || !status.TailscaleConnected {
		t.Fatalf("ConnectTailscale() = (%+v, %v), LAN choice was not preserved", status, err)
	}
}

func TestCreateAdministratorUsesNativeBoundariesInOrder(t *testing.T) {
	service, calls := testService(nil, false, nil, true)
	request := AdministratorRequest{Username: "ada", Password: "correct horse battery", AuthorizedKey: "ssh-ed25519 AAAA"}
	status, err := service.CreateAdministrator(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{"accounts.prepare:ada", "forgejo.prepare:ada", "accounts.promote:ada"}
	if !reflect.DeepEqual(*calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", *calls, wantCalls)
	}
	if !status.CanDismiss || len(status.Administrators) != 1 || !status.Administrators[0].ForgejoReady {
		t.Fatalf("status = %+v", status)
	}
}

func TestCreateAdministratorRetainsAndReportsPartialState(t *testing.T) {
	service, calls := testService(nil, false, nil, true)
	forgejo := service.Forgejo.(*fakeForgejo)
	forgejo.prepareErr = errors.New("Forgejo unavailable")
	status, err := service.CreateAdministrator(context.Background(), AdministratorRequest{Username: "ada"})
	if err == nil || status.Dismissed {
		t.Fatalf("CreateAdministrator() = (%+v, %v)", status, err)
	}
	wantCalls := []string{"accounts.prepare:ada", "forgejo.prepare:ada"}
	if !reflect.DeepEqual(*calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", *calls, wantCalls)
	}
}
