package observe

import (
	"context"
	"errors"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
)

func TestParseAcceptedAndDisconnectedIPv4IPv6(t *testing.T) {
	user, address, port, fingerprint, ok := ParseAccepted("Accepted publickey for soda-p-demo from 192.0.2.4 port 54321 ssh2: ED25519 SHA256:key")
	if !ok || user != "soda-p-demo" || address != "192.0.2.4" || port != 54321 || fingerprint != "SHA256:key" {
		t.Fatalf("unexpected accepted parse: %q %q %d %q %t", user, address, port, fingerprint, ok)
	}
	user, address, port, ok = ParseDisconnected("Disconnected from user soda-p-demo 2001:db8::4 port 54321")
	if !ok || user != "soda-p-demo" || address != "2001:db8::4" || port != 54321 {
		t.Fatalf("unexpected disconnected parse: %q %q %d %t", user, address, port, ok)
	}
}

func TestParseJournalLifecycleAndMalformedProjectEvents(t *testing.T) {
	projects := map[string]domain.Project{"soda-p-demo": {ID: "p1", UnixUser: "soda-p-demo"}}
	people := map[string]domain.Person{"SHA256:key": {ID: "person"}}
	active, malformed := ParseJournal([]byte("{\"MESSAGE\":\"Accepted publickey for soda-p-demo from 192.0.2.4 port 5000 ssh2: ED25519 SHA256:key\",\"__REALTIME_TIMESTAMP\":\"1000000\"}\n{\"MESSAGE\":\"Accepted publickey for root from 192.0.2.4 port 22 ssh2: ED25519 SHA256:key\"}\n"), projects, people)
	if malformed || len(active) != 1 {
		t.Fatalf("expected one Soda connection only: malformed=%t active=%#v", malformed, active)
	}
	active, malformed = ParseJournal([]byte("{\"MESSAGE\":\"Accepted publickey for soda-p-demo malformed\"}\n"), projects, people)
	if !malformed || len(active) != 0 {
		t.Fatalf("malformed Soda journal event must degrade: %t %#v", malformed, active)
	}
}

func TestSocketsAndChannelsIncludeTransportOnly(t *testing.T) {
	active := map[string]journalConnection{
		connectionKey("soda-p-demo", "2001:db8::4", 5000): {projectID: "p", personID: "a", projectUser: "soda-p-demo", clientAddress: "2001:db8::4", clientPort: 5000},
	}
	sockets := ParseEstablishedSockets([]byte("ESTAB 0 0 [2001:db8::1]:22 [2001:db8::4]:5000\n"))
	if len(sockets) != 1 || sockets[0].RemoteAddress != "2001:db8::4" {
		t.Fatalf("failed IPv6 socket parsing: %#v", sockets)
	}
	connections := activeConnections(reconcileSockets(active, sockets), sockets, true, nil)
	if len(connections) != 1 || len(connections[0].Channels) != 0 {
		t.Fatalf("socket without gateway process must be transport-only: %#v", connections)
	}
	people := []domain.Person{{ID: "a", Username: "alice"}}
	worktrees := []domain.Worktree{{ID: "w", PersonID: "a", Path: "/srv/p/worktrees/alice"}}
	channels := processChannels([][]string{{"SODA_ACTOR=alice", "SODA_WORKTREE=/srv/p/worktrees/alice", "SSH_CONNECTION=2001:db8::4 5000 2001:db8::1 22", "SSH_ORIGINAL_COMMAND=internal-sftp"}}, people, worktrees)
	connections = activeConnections(active, sockets, true, channels)
	if got := connections[0].Channels; len(got) != 1 || got[0].Kind != domain.SSHChannelSFTP || got[0].WorktreeID != "w" {
		t.Fatalf("failed gateway channel attribution: %#v", got)
	}
}

type fakeSessionSystem struct {
	journal    []byte
	sockets    []Socket
	envs       [][]string
	journalErr error
	socketErr  error
	envErr     error
}

func (f fakeSessionSystem) Journal(context.Context) ([]byte, error) { return f.journal, f.journalErr }
func (f fakeSessionSystem) Sockets(context.Context) ([]Socket, error) {
	return f.sockets, f.socketErr
}
func (f fakeSessionSystem) ProcessEnvironments(context.Context) ([][]string, error) {
	return f.envs, f.envErr
}

func TestSessionInspectorDegradesOnSocketFailureButRetainsJournalState(t *testing.T) {
	key := "ssh-ed25519 AQID"
	fingerprint, err := domain.SSHKeyFingerprint(key)
	if err != nil {
		t.Fatal(err)
	}
	system := fakeSessionSystem{
		journal:   []byte("{\"MESSAGE\":\"Accepted publickey for soda-p-demo from 192.0.2.4 port 5000 ssh2: ED25519 " + fingerprint + "\"}\n"),
		socketErr: errors.New("ss unavailable"),
	}
	inspector := NewSystemSessionInspector(system)
	observation, err := inspector.Inspect(context.Background(), []domain.Project{{ID: "p", UnixUser: "soda-p-demo"}}, []domain.Person{{ID: "a", SSHPublicKey: key}}, nil)
	if err != nil || !observation.Degraded || len(observation.Connections) != 1 {
		t.Fatalf("socket failure must preserve journal connection as degraded telemetry: %#v %v", observation, err)
	}
}
