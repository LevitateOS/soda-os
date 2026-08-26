package observe

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

type Socket struct {
	LocalAddress  string
	LocalPort     uint16
	RemoteAddress string
	RemotePort    uint16
}

// SessionSystem abstracts current-boot journal, established sockets, and
// gateway process environments. It intentionally returns observations rather
// than exposing mutable process handles.
type SessionSystem interface {
	Journal(context.Context) ([]byte, error)
	Sockets(context.Context) ([]Socket, error)
	ProcessEnvironments(context.Context) ([][]string, error)
}

type LinuxSessionSystem struct{}

func (LinuxSessionSystem) Journal(ctx context.Context) ([]byte, error) {
	return runCommand(ctx, "journalctl", "--boot", "--unit=sshd.service", "--output=json", "--no-pager")
}

func (LinuxSessionSystem) Sockets(ctx context.Context) ([]Socket, error) {
	output, err := runCommand(ctx, "ss", "-Htn", "state", "established")
	if err != nil {
		return nil, err
	}
	return ParseEstablishedSockets(output), nil
}

func (LinuxSessionSystem) ProcessEnvironments(context.Context) ([][]string, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	result := make([][]string, 0)
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		contents, readErr := os.ReadFile(filepath.Join("/proc", entry.Name(), "environ"))
		if readErr != nil {
			continue
		}
		fields := strings.Split(string(contents), "\x00")
		result = append(result, fields)
	}
	return result, nil
}

type SystemSessionInspector struct{ System SessionSystem }

func NewSystemSessionInspector(system SessionSystem) *SystemSessionInspector {
	if system == nil {
		system = LinuxSessionSystem{}
	}
	return &SystemSessionInspector{System: system}
}

func (s *SystemSessionInspector) Inspect(ctx context.Context, projects []domain.Project, people []domain.Person, worktrees []domain.Worktree) (SessionObservation, error) {
	journal, err := s.System.Journal(ctx)
	if err != nil {
		return SessionObservation{}, annotate(err, "read sshd journal")
	}
	projectByUser := make(map[string]domain.Project, len(projects))
	for _, project := range projects {
		projectByUser[project.UnixUser] = project
	}
	personByFingerprint := make(map[string]domain.Person, len(people))
	for _, person := range people {
		if fingerprint := PublicKeyFingerprint(person.SSHPublicKey); fingerprint != "" {
			personByFingerprint[fingerprint] = person
		}
	}
	active, malformed := ParseJournal(journal, projectByUser, personByFingerprint)
	if malformed {
		return SessionObservation{}, ErrJournalFormat
	}
	sockets, socketErr := s.System.Sockets(ctx)
	degraded := socketErr != nil
	if socketErr == nil {
		active = reconcileSockets(active, sockets)
	}
	environments, environmentErr := s.System.ProcessEnvironments(ctx)
	if environmentErr != nil {
		degraded = true
	}
	channels := processChannels(environments, people, worktrees)
	connections := activeConnections(active, sockets, socketErr == nil, channels)
	return SessionObservation{Connections: connections, Degraded: degraded}, nil
}

type journalConnection struct {
	projectID     string
	personID      string
	projectUser   string
	connectedAt   time.Time
	clientAddress string
	clientPort    uint16
}

type journalEntry struct {
	Message   string `json:"MESSAGE"`
	Timestamp string `json:"__REALTIME_TIMESTAMP"`
}

// ParseJournal applies accepted/disconnected events to project-account
// connections. A malformed event for a Soda project is an observer failure,
// not evidence that the connection list is empty.
func ParseJournal(data []byte, projects map[string]domain.Project, people map[string]domain.Person) (map[string]journalConnection, bool) {
	active := make(map[string]journalConnection)
	malformed := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry journalEntry
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		if strings.HasPrefix(entry.Message, "Accepted publickey for soda-p-") {
			user, address, port, fingerprint, ok := ParseAccepted(entry.Message)
			if !ok {
				malformed = true
				continue
			}
			project, projectOK := projects[user]
			person, personOK := people[fingerprint]
			if !projectOK || !personOK {
				continue
			}
			active[connectionKey(user, address, port)] = journalConnection{
				projectID: project.ID, personID: person.ID, projectUser: user,
				connectedAt: parseJournalTime(entry.Timestamp), clientAddress: address, clientPort: port,
			}
			continue
		}
		if strings.HasPrefix(entry.Message, "Disconnected from user soda-p-") {
			user, address, port, ok := ParseDisconnected(entry.Message)
			if !ok {
				malformed = true
				continue
			}
			delete(active, connectionKey(user, address, port))
		}
	}
	return active, malformed
}

func ParseAccepted(message string) (user, address string, port uint16, fingerprint string, ok bool) {
	rest, found := strings.CutPrefix(message, "Accepted publickey for ")
	if !found {
		return "", "", 0, "", false
	}
	user, rest, found = strings.Cut(rest, " from ")
	if !found {
		return "", "", 0, "", false
	}
	address, rest, found = strings.Cut(rest, " port ")
	if !found {
		return "", "", 0, "", false
	}
	portText, details, found := strings.Cut(rest, " ssh2: ")
	if !found {
		return "", "", 0, "", false
	}
	parsedPort, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return "", "", 0, "", false
	}
	fields := strings.Fields(details)
	if len(fields) == 0 {
		return "", "", 0, "", false
	}
	return user, address, uint16(parsedPort), fields[len(fields)-1], true
}

func ParseDisconnected(message string) (user, address string, port uint16, ok bool) {
	rest, found := strings.CutPrefix(message, "Disconnected from user ")
	if !found {
		return "", "", 0, false
	}
	fields := strings.Fields(rest)
	if len(fields) != 4 || fields[2] != "port" {
		return "", "", 0, false
	}
	parsedPort, err := strconv.ParseUint(fields[3], 10, 16)
	if err != nil {
		return "", "", 0, false
	}
	return fields[0], fields[1], uint16(parsedPort), true
}

func PublicKeyFingerprint(key string) string {
	fields := strings.Fields(key)
	if len(fields) < 2 || (!strings.HasPrefix(fields[0], "ssh-") && !strings.HasPrefix(fields[0], "ecdsa-")) {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(decoded)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(digest[:])
}

func parseJournalTime(value string) time.Time {
	microseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.UnixMicro(microseconds)
}

func reconcileSockets(active map[string]journalConnection, sockets []Socket) map[string]journalConnection {
	present := make(map[string]struct{}, len(sockets))
	for _, socket := range sockets {
		if socket.LocalPort == 22 {
			present[endpointKey(socket.RemoteAddress, socket.RemotePort)] = struct{}{}
		}
	}
	for key, connection := range active {
		if _, ok := present[endpointKey(connection.clientAddress, connection.clientPort)]; !ok {
			delete(active, key)
		}
	}
	return active
}

func activeConnections(active map[string]journalConnection, sockets []Socket, socketsAvailable bool, channels map[string][]domain.SSHChannel) []domain.ActiveSSHConnection {
	socketByPeer := make(map[string]Socket, len(sockets))
	for _, socket := range sockets {
		if socket.LocalPort == 22 {
			socketByPeer[endpointKey(socket.RemoteAddress, socket.RemotePort)] = socket
		}
	}
	connections := make([]domain.ActiveSSHConnection, 0, len(active))
	for _, connection := range active {
		serverAddress, serverPort := "soda", uint16(22)
		if socketsAvailable {
			socket := socketByPeer[endpointKey(connection.clientAddress, connection.clientPort)]
			serverAddress, serverPort = socket.LocalAddress, socket.LocalPort
		}
		connections = append(connections, domain.ActiveSSHConnection{
			ID: connectionKey(connection.projectUser, connection.clientAddress, connection.clientPort), ProjectID: connection.projectID,
			PersonID: connection.personID, ConnectedAt: connection.connectedAt, ClientAddress: connection.clientAddress,
			ClientPort: connection.clientPort, ServerAddress: serverAddress, ServerPort: serverPort,
			Channels: append([]domain.SSHChannel(nil), channels[endpointKey(connection.clientAddress, connection.clientPort)]...),
		})
	}
	sort.Slice(connections, func(i, j int) bool {
		if connections[i].ProjectID == connections[j].ProjectID {
			return connections[i].ConnectedAt.Before(connections[j].ConnectedAt)
		}
		return connections[i].ProjectID < connections[j].ProjectID
	})
	return connections
}

func processChannels(environments [][]string, people []domain.Person, worktrees []domain.Worktree) map[string][]domain.SSHChannel {
	personByUsername := make(map[string]string, len(people))
	for _, person := range people {
		personByUsername[person.Username] = person.ID
	}
	worktreeByPath := make(map[string]domain.Worktree, len(worktrees))
	for _, worktree := range worktrees {
		worktreeByPath[worktree.Path] = worktree
	}
	seen := make(map[string]map[domain.SSHChannel]struct{})
	for _, fields := range environments {
		env := environmentMap(fields)
		actor, worktreePath, connection := env["SODA_ACTOR"], env["SODA_WORKTREE"], env["SSH_CONNECTION"]
		worktree, exists := worktreeByPath[worktreePath]
		if actor == "" || connection == "" || !exists || personByUsername[actor] != worktree.PersonID {
			continue
		}
		parts := strings.Fields(connection)
		if len(parts) < 2 {
			continue
		}
		port, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			continue
		}
		kind := domain.SSHChannelInteractive
		if env["SSH_ORIGINAL_COMMAND"] == "internal-sftp" {
			kind = domain.SSHChannelSFTP
		} else if env["SSH_ORIGINAL_COMMAND"] != "" {
			kind = domain.SSHChannelCommand
		}
		key := endpointKey(parts[0], uint16(port))
		if seen[key] == nil {
			seen[key] = make(map[domain.SSHChannel]struct{})
		}
		seen[key][domain.SSHChannel{Kind: kind, WorktreeID: worktree.ID}] = struct{}{}
	}
	result := make(map[string][]domain.SSHChannel, len(seen))
	for key, values := range seen {
		for channel := range values {
			result[key] = append(result[key], channel)
		}
		sort.Slice(result[key], func(i, j int) bool {
			if result[key][i].WorktreeID == result[key][j].WorktreeID {
				return result[key][i].Kind < result[key][j].Kind
			}
			return result[key][i].WorktreeID < result[key][j].WorktreeID
		})
	}
	return result
}

func environmentMap(fields []string) map[string]string {
	result := make(map[string]string, len(fields))
	for _, field := range fields {
		if key, value, ok := strings.Cut(field, "="); ok {
			result[key] = value
		}
	}
	return result
}

func ParseEstablishedSockets(output []byte) []Socket {
	var sockets []Socket
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		local, localOK := parseEndpoint(fields[len(fields)-2])
		remote, remoteOK := parseEndpoint(fields[len(fields)-1])
		if !localOK || !remoteOK {
			continue
		}
		sockets = append(sockets, Socket{LocalAddress: local.host, LocalPort: local.port, RemoteAddress: remote.host, RemotePort: remote.port})
	}
	return sockets
}

type endpoint struct {
	host string
	port uint16
}

func parseEndpoint(value string) (endpoint, bool) {
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		// ss emits IPv4 addresses without brackets, which SplitHostPort accepts;
		// retain a final split for unusual wildcard formatting.
		host, portText, _ = strings.Cut(value, ":")
		if host == "" || portText == "" {
			return endpoint{}, false
		}
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return endpoint{}, false
	}
	return endpoint{host: strings.Trim(host, "[]"), port: uint16(port)}, true
}

func endpointKey(address string, port uint16) string { return address + "|" + strconv.Itoa(int(port)) }
func connectionKey(user, address string, port uint16) string {
	return user + "|" + endpointKey(address, port)
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

var _ SessionInspector = (*SystemSessionInspector)(nil)
