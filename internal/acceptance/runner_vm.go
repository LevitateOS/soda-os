package acceptance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func (state *runnerState) installAndOnboard(ctx context.Context) (Remote, *VM, error) {
	tailnet, err := NewTailnet()
	if err != nil {
		return Remote{}, nil, err
	}
	state.tailnet = tailnet
	before, raw, err := tailnet.Snapshot(ctx)
	if err != nil {
		return Remote{}, nil, err
	}
	if err = state.evidence.Write("iso/host-tailnet-before.json", raw); err != nil {
		return Remote{}, nil, err
	}
	return state.completeISOFlow(ctx, before)
}

func (state *runnerState) completeISOFlow(ctx context.Context, before tailnetStatus) (Remote, *VM, error) {
	vm, err := state.launch(ctx, "iso/install", "install", state.paths.installedDisk, state.artifacts.CandidateISO)
	if err != nil {
		return Remote{}, nil, err
	}
	fmt.Fprintf(state.output, "Complete stock graphical Anaconda and Soda Setup in the QEMU display. Use administrator %q and the protected values in:\n  password: %s\n  SSH public key: %s\n  one-use Tailscale key: %s\n", state.options.Administrator.Username, state.paths.password, state.paths.adminPublicKey, state.options.TailscaleKey)
	host, raw, err := state.resolveGuest(ctx, before)
	if err != nil {
		return Remote{}, vm, err
	}
	if err = state.evidence.Write("iso/host-tailnet-enrolled.json", raw); err != nil {
		return Remote{}, vm, err
	}
	if err = state.evidence.Write("iso/tailnet-address.txt", []byte(host+"\n")); err != nil {
		return Remote{}, vm, err
	}
	remote := state.tailnetRemote(host)
	password, err := os.ReadFile(state.paths.password)
	if err != nil {
		return Remote{}, vm, err
	}
	if err = state.registerTailnetCleanup(remote, password); err != nil {
		return Remote{}, vm, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err = remote.WaitReady(waitCtx); err != nil {
		return Remote{}, vm, err
	}
	return state.completeInstalledAccess(ctx, remote, vm, password)
}

func (state *runnerState) completeInstalledAccess(ctx context.Context, remote Remote, vm *VM, password []byte) (Remote, *VM, error) {
	if err := state.verifyForwardedIngressRejected(ctx); err != nil {
		return Remote{}, vm, err
	}
	if err := remote.Sudo(ctx, password, "/usr/libexec/soda/soda-setup status\n", "iso/setup-status"); err != nil {
		return Remote{}, vm, err
	}
	if err := state.enableAndVerifyLAN(ctx, remote, password); err != nil {
		return Remote{}, vm, err
	}
	return remote, vm, state.captureQMP(ctx, vm, "iso/qmp-running.json")
}

func (state *runnerState) registerTailnetCleanup(remote Remote, password []byte) error {
	attempted := false
	state.logout = func(ctx context.Context) error {
		if attempted {
			return nil
		}
		attempted = true
		return remote.Sudo(ctx, password, "/usr/bin/tailscale logout\n", "cleanup/tailscale-logout")
	}
	if err := state.cleanup.Add(CleanupAction{Name: "guest Tailnet enrollment", Run: state.logout}); err != nil {
		logoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return errors.Join(err, state.logout(logoutCtx))
	}
	return nil
}

func (state *runnerState) verifyForwardedIngressRejected(ctx context.Context) error {
	sshCtx, cancelSSH := context.WithTimeout(ctx, 6*time.Second)
	sshOutput, sshErr := CommandOutput(sshCtx, CommandSpec{Name: "ssh-keyscan", Args: []string{
		"-T", "5", "-t", "ed25519", "-p", fmt.Sprint(state.options.Ports.SSH), "127.0.0.1",
	}})
	cancelSSH()
	if sshErr == nil && len(sshOutput) != 0 {
		return fmt.Errorf("default-drop guest exposed SSH through the host-forwarded interface")
	}
	cockpitCtx, cancelCockpit := context.WithTimeout(ctx, 6*time.Second)
	defer cancelCockpit()
	err := RunCommand(cockpitCtx, CommandSpec{Name: "curl", Args: []string{
		"--fail", "--silent", "--show-error", "--insecure", "--max-time", "5",
		"https://127.0.0.1:" + fmt.Sprint(state.options.Ports.Cockpit) + "/ping",
	}})
	if err == nil {
		return errors.New("default-drop guest exposed Cockpit through the host-forwarded interface")
	}
	return state.evidence.Write("iso/public-ingress-rejection.txt", []byte("ssh=rejected\ncockpit=rejected\n"))
}

func (state *runnerState) enableAndVerifyLAN(ctx context.Context, tailnet Remote, password []byte) error {
	script := `status=$(/usr/libexec/soda/soda-setup status)
connection=$(jq -er 'first(.connections[] | select(.name != "tailscale0") | .name)' <<<"$status")
jq -nc --arg connection "$connection" '{connection:$connection}' | /usr/libexec/soda/soda-setup allow-local-network
`
	if err := tailnet.Sudo(ctx, password, script, "iso/enable-lan-after-tailscale"); err != nil {
		return err
	}
	local := Remote{
		Username: state.options.Administrator.Username, Host: "127.0.0.1", Port: state.options.Ports.SSH,
		CockpitPort: state.options.Ports.Cockpit, Key: state.paths.adminKey,
		KnownHosts: filepath.Join(state.paths.work, "iso-lan-known-hosts"), Evidence: state.evidence,
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := local.WaitReady(waitCtx); err != nil {
		return fmt.Errorf("verify LAN after Tailscale: %w", err)
	}
	return local.Capture(ctx, "iso/lan-after-tailscale", []byte(localAccessCheck), "/bin/bash", "-s")
}

func (state *runnerState) tailnetRemote(host string) Remote {
	return Remote{
		Username: state.options.Administrator.Username, Host: host, Port: 22, CockpitPort: 9090,
		Key: state.paths.adminKey, KnownHosts: state.paths.knownHosts, Evidence: state.evidence,
	}
}

func (state *runnerState) launch(ctx context.Context, relative, mode, disk, iso string) (*VM, error) {
	directory, err := state.evidence.path(relative)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	config := VMConfig{
		Architecture: nativeArchitecture(), Mode: mode, Disk: disk, ISO: iso,
		Directory: directory, DiskSize: state.options.DiskSize, Host: "127.0.0.1",
		SSHPort: state.options.Ports.SSH, CockpitPort: state.options.Ports.Cockpit,
	}
	vm, err := LaunchVM(ctx, config)
	if err != nil {
		return nil, err
	}
	if err = state.cleanup.Add(CleanupAction{Name: "QEMU " + relative, Run: vm.Stop}); err != nil {
		stopCtx, cancel := StopDeadline()
		defer cancel()
		return nil, errors.Join(err, vm.Stop(stopCtx))
	}
	return vm, nil
}

func (state *runnerState) resolveGuest(ctx context.Context, before tailnetStatus) (string, []byte, error) {
	discoveryCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	return state.tailnet.Discover(discoveryCtx, before)
}

func (state *runnerState) captureQMP(ctx context.Context, vm *VM, relative string) error {
	var status map[string]any
	if err := vm.QMP.Execute(ctx, "query-status", "status", nil, &status); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return state.evidence.Write(relative, append(contents, '\n'))
}

func (state *runnerState) exerciseReusableQCOW2(ctx context.Context) error {
	if err := cloneDisk(ctx, state.artifacts.CandidateQCOW2, state.paths.qcowDisk); err != nil {
		return err
	}
	vm, err := state.launch(ctx, "qcow2/first-boot", "qcow2", state.paths.qcowDisk, "")
	if err != nil {
		return err
	}
	fmt.Fprintf(state.output, "Complete Soda Setup for the reusable QCOW2 in the QEMU display. Use administrator %q, the protected password at %s, the public key at %s, and allow local-network access.\n", state.options.Administrator.Username, state.paths.password, state.paths.adminPublicKey)
	knownHosts := filepath.Join(state.paths.work, "qcow-known-hosts")
	remote := Remote{
		Username: state.options.Administrator.Username, Host: "127.0.0.1", Port: state.options.Ports.SSH,
		CockpitPort: state.options.Ports.Cockpit, Key: state.paths.adminKey, KnownHosts: knownHosts, Evidence: state.evidence,
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err = remote.WaitReady(waitCtx); err != nil {
		return err
	}
	if err = state.runQCOW2Checks(ctx, remote); err != nil {
		return err
	}
	if err = state.captureQMP(ctx, vm, "qcow2/qmp-running.json"); err != nil {
		return err
	}
	return vm.PowerDown(ctx)
}

func cloneDisk(ctx context.Context, source, destination string) error {
	if runtime.GOOS == "linux" {
		return RunCommand(ctx, CommandSpec{Name: "cp", Args: []string{"--reflink=auto", "--", source, destination}})
	}
	return copyFile(source, destination)
}
