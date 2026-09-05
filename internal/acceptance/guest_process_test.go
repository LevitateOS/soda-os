package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// A native test subprocess speaks just QMP. The production LaunchVM, Process,
// powerdown, socket, and log-file ownership paths run unchanged, without QEMU.
func TestGuestQEMUProcess(t *testing.T) {
	if os.Getenv("SODA_GUEST_TEST_CHILD") != "1" {
		return
	}
	var socket, label string
	for index, arg := range os.Args {
		switch arg {
		case "-qmp":
			socket = strings.Split(strings.TrimPrefix(os.Args[index+1], "unix:"), ",")[0]
		case "-serial":
			label = filepath.Base(filepath.Dir(strings.TrimPrefix(os.Args[index+1], "file:")))
		}
	}
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	defer listener.Close()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		guestProcessEvent("stop:" + label)
		listener.Close()
	}()
	guestProcessEvent("start:" + label)
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		command, err := serveGuestQMP(connection)
		connection.Close()
		require.NoError(t, err)
		if command == "system_powerdown" {
			guestProcessEvent("powerdown:" + label)
			return
		}
	}
}

func serveGuestQMP(connection net.Conn) (string, error) {
	decoder, encoder := json.NewDecoder(connection), json.NewEncoder(connection)
	if err := encoder.Encode(map[string]any{"QMP": map[string]any{}}); err != nil {
		return "", err
	}
	var request qmpRequest
	for range 2 {
		if err := decoder.Decode(&request); err != nil {
			return "", err
		}
		if err := encoder.Encode(qmpResponse{Return: json.RawMessage(`{"status":"running"}`), ID: request.ID}); err != nil {
			return "", err
		}
	}
	return request.Execute, nil
}

func guestProcessEvent(event string) {
	file, err := os.OpenFile(os.Getenv("GUEST_EVENT_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	_, err = fmt.Fprintln(file, event)
	closeErr := file.Close()
	if err != nil {
		panic(err)
	}
	if closeErr != nil {
		panic(closeErr)
	}
}

func prepareGuestCommands(t *testing.T) Evidence {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("GUEST_TEST_BINARY", executable)
	t.Setenv("SODA_GUEST_TEST_CHILD", "1")
	t.Setenv("GUEST_EVENT_LOG", filepath.Join(t.TempDir(), "events"))
	installAcceptanceCommand(t, "fixture-qemu", `GORACE=atexit_sleep_ms=0 exec "$GUEST_TEST_BINARY" -test.run='^TestGuestQEMUProcess$' -- "$@"
`)
	t.Setenv("SODA_QEMU", "fixture-qemu")
	installAcceptanceCommand(t, "ssh", `cat >/dev/null
printf 'logout\n' >>"$GUEST_EVENT_LOG"
exit "${FAIL_GUEST_LOGOUT:-0}"
`)
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	return evidence
}

func guestTestConfig(t *testing.T, evidence Evidence, label string) VMConfig {
	t.Helper()
	directory := filepath.Join(evidence.Root, label)
	require.NoError(t, os.Mkdir(directory, 0700))
	disk := filepath.Join(directory, "disk.qcow2")
	require.NoError(t, os.WriteFile(disk, []byte("fixture-disk"), 0600))
	vars := filepath.Join(directory, "firmware-vars")
	require.NoError(t, os.WriteFile(vars, []byte("fixture-vars"), 0600))
	t.Setenv("SODA_QEMU_VARS", vars)
	return VMConfig{Architecture: nativeArchitecture(), Mode: "qcow2", Disk: disk, Directory: directory, Host: "127.0.0.1", SSHPort: 2222, CockpitPort: 19090, ForgejoPort: 13000}
}

func enrolledTestGuest(t *testing.T, evidence Evidence, cleanup *Cleanup) *guest {
	t.Helper()
	guest, err := launchGuest(context.Background(), guestTestConfig(t, evidence, "iso"), evidence, cleanup)
	require.NoError(t, err)
	t.Cleanup(func() { _ = guest.cleanup(context.Background()) })
	guest.enrollment = &guestEnrollment{remote: Remote{Evidence: evidence}, password: []byte("fixture-password")}
	return guest
}

func guestEvents(t *testing.T) []string {
	t.Helper()
	contents, err := os.ReadFile(os.Getenv("GUEST_EVENT_LOG"))
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(contents)), "\n")
}
