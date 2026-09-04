package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/tailnet"
	"golang.org/x/sys/unix"
)

type TailnetStatus interface {
	Status(context.Context) (tailnet.Status, error)
}

type NativeNetwork struct {
	Runner  linuxhost.CommandRunner
	Tailnet TailnetStatus
}

func (network NativeNetwork) dependencies() (linuxhost.CommandRunner, TailnetStatus) {
	runner := network.Runner
	if runner == nil {
		runner = linuxhost.ExecCommandRunner{}
	}
	client := network.Tailnet
	if client == nil {
		client = tailnet.New(tailnet.Options{})
	}
	return runner, client
}

func (network NativeNetwork) Status(ctx context.Context) ([]Connection, bool, error) {
	runner, client := network.dependencies()
	result, err := runner.Run(ctx, linuxhost.Command{Name: "/usr/bin/nmcli", Args: []string{"--get-values", "NAME", "connection", "show", "--active"}})
	if err != nil {
		return nil, false, err
	}
	if result.ExitCode != 0 {
		return nil, false, fmt.Errorf("list active NetworkManager connections: %s", strings.TrimSpace(result.Stderr))
	}
	connections := make([]Connection, 0)
	for _, name := range activeConnectionNames(result.Stdout) {
		connection, include, inspectErr := inspectNetworkManagerConnection(ctx, runner, name)
		if inspectErr != nil {
			return nil, false, inspectErr
		}
		if include {
			connections = append(connections, connection)
		}
	}
	sort.Slice(connections, func(i, j int) bool { return connections[i].Name < connections[j].Name })
	status, statusErr := client.Status(ctx)
	connected := statusErr == nil && status.EnrollmentState() == tailnet.Enrolled
	return connections, connected, nil
}

func activeConnectionNames(output string) []string {
	names := make([]string, 0)
	seen := map[string]bool{}
	for _, value := range strings.Split(strings.TrimSpace(output), "\n") {
		name := strings.TrimSpace(value)
		if name == "" || strings.ContainsAny(name, "\x00\r\n") || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func inspectNetworkManagerConnection(ctx context.Context, runner linuxhost.CommandRunner, name string) (Connection, bool, error) {
	connectionType, err := runner.Run(ctx, linuxhost.Command{Name: "/usr/bin/nmcli", Args: []string{"--get-values", "connection.type", "connection", "show", name}})
	if err != nil {
		return Connection{}, false, err
	}
	if connectionType.ExitCode != 0 {
		return Connection{}, false, fmt.Errorf("inspect NetworkManager connection %s: %s", name, strings.TrimSpace(connectionType.Stderr))
	}
	if strings.TrimSpace(connectionType.Stdout) == "loopback" {
		return Connection{}, false, nil
	}
	zone, err := runner.Run(ctx, linuxhost.Command{Name: "/usr/bin/nmcli", Args: []string{"--get-values", "connection.zone", "connection", "show", name}})
	if err != nil {
		return Connection{}, false, err
	}
	return Connection{Name: name, LocalNetworkAllowed: zone.ExitCode == 0 && strings.TrimSpace(zone.Stdout) == "trusted"}, true, nil
}

func (network NativeNetwork) AllowLocalNetwork(ctx context.Context, connection string) error {
	if strings.TrimSpace(connection) == "" || strings.ContainsAny(connection, "\x00\r\n") {
		return errors.New("an active NetworkManager connection is required")
	}
	runner, _ := network.dependencies()
	result, err := runner.Run(ctx, linuxhost.Command{Name: "/usr/bin/soda-local-access", Args: []string{connection, "on"}})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("allow local-network access on %s: %s", connection, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (network NativeNetwork) ConnectTailscale(ctx context.Context, authKey string) error {
	if err := validateTailscaleAuthKey(authKey); err != nil {
		return err
	}
	secret, err := sealedSecret([]byte(authKey))
	if err != nil {
		return fmt.Errorf("prepare one-use Tailscale key: %w", err)
	}
	defer secret.Close()
	runner, client := network.dependencies()
	if err = enrollTailscale(ctx, runner, secret); err != nil {
		return err
	}
	if err = verifyTailscaleConnected(ctx, client); err != nil {
		return err
	}
	return restartForgejoForTailnet(ctx, runner)
}

func validateTailscaleAuthKey(authKey string) error {
	if authKey == "" || len(authKey) > 4096 || strings.ContainsAny(authKey, "\x00\r\n") {
		return errors.New("a valid one-line Tailscale auth key is required")
	}
	return nil
}

func enrollTailscale(ctx context.Context, runner linuxhost.CommandRunner, secret *os.File) error {
	result, err := runner.Run(ctx, linuxhost.Command{
		Name: "/usr/bin/tailscale", Args: []string{"up", "--auth-key=file:/proc/self/fd/3"}, ExtraFiles: []*os.File{secret},
	})
	if err != nil {
		return errors.New("Tailscale enrollment could not be started")
	}
	if result.ExitCode != 0 {
		return errors.New("Tailscale rejected the one-use auth key")
	}
	return nil
}

func verifyTailscaleConnected(ctx context.Context, client TailnetStatus) error {
	status, err := client.Status(ctx)
	if err != nil || status.EnrollmentState() != tailnet.Enrolled {
		return errors.New("Tailscale did not report a connected appliance")
	}
	return nil
}

func restartForgejoForTailnet(ctx context.Context, runner linuxhost.CommandRunner) error {
	result, err := runner.Run(ctx, linuxhost.Command{Name: "/usr/bin/systemctl", Args: []string{"restart", "forgejo.service"}})
	if err != nil || result.ExitCode != 0 {
		return errors.New("Tailscale connected, but Forgejo could not reload its Tailnet address")
	}
	return nil
}

func sealedSecret(contents []byte) (*os.File, error) {
	descriptor, err := unix.MemfdCreate("soda-setup-tailscale", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), "soda-setup-tailscale")
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, errors.New("open anonymous Tailscale key descriptor")
	}
	if _, err = file.Write(contents); err != nil {
		file.Close()
		return nil, err
	}
	if _, err = file.Seek(0, 0); err != nil {
		file.Close()
		return nil, err
	}
	seals := unix.F_SEAL_SEAL | unix.F_SEAL_SHRINK | unix.F_SEAL_GROW | unix.F_SEAL_WRITE
	if _, err = unix.FcntlInt(file.Fd(), unix.F_ADD_SEALS, seals); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}
