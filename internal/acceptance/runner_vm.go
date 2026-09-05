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

func (state *runnerState) installAndOnboard(ctx context.Context, admin personFixture) (string, *VM, error) {
	tailnet, err := NewTailnet()
	if err != nil {
		return "", nil, err
	}
	before, raw, err := tailnet.Snapshot(ctx)
	if err != nil {
		return "", nil, err
	}
	if err = state.evidence.Write("iso/host-tailnet-before.json", raw); err != nil {
		return "", nil, err
	}
	return state.completeISOFlow(ctx, before, tailnet, admin)
}

func (state *runnerState) completeISOFlow(ctx context.Context, before tailnetStatus, tailnet Tailnet, admin personFixture) (string, *VM, error) {
	vm, err := state.launch(ctx, "iso/install", "install", state.paths.installedDisk, state.artifacts.CandidateISO)
	if err != nil {
		return "", nil, err
	}
	fmt.Fprintf(state.output, "Create Linux administrator %q through graphical Anaconda, reboot, and log in normally. Add the personal public key through Cockpit Accounts before continuing. Protected input paths:\n  password: %s\n  SSH public key: %s\n\n", admin.Remote.Username, state.paths.password, state.paths.adminPublicKey)
	if err = state.checks.record("iso-first-boot-defaults", verifyInitialLocalForwardedAccess(ctx, admin, state.options.Ports.Forgejo)); err != nil {
		return "", vm, err
	}
	fmt.Fprintln(state.output, "Local-forwarded access through QEMU is verified (not independent LAN evidence). Open Cockpit → Tailscale and sign in through its native browser authentication URL.")
	host, raw, err := state.resolveGuest(ctx, before, tailnet)
	if err != nil {
		return "", vm, err
	}
	if err = state.evidence.Write("iso/host-tailnet-enrolled.json", raw); err != nil {
		return "", vm, err
	}
	if err = state.evidence.Write("iso/tailnet-address.txt", []byte(host+"\n")); err != nil {
		return "", vm, err
	}
	remote := admin.Remote
	remote.Host, remote.Port, remote.CockpitPort = host, 22, 9090
	remote.KnownHosts = state.paths.knownHosts
	if err = state.registerTailnetCleanup(&remote, admin.LinuxPassword); err != nil {
		return "", vm, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err = remote.WaitReady(waitCtx); err != nil {
		return "", vm, err
	}
	if err = state.checks.record("local-forwarded-access", state.verifyLocalForwardedAccess(ctx, admin, remote)); err != nil {
		return "", vm, err
	}
	return host, vm, state.captureQMP(ctx, vm, "iso/qmp-running.json")
}

func (state *runnerState) registerTailnetCleanup(remote *Remote, password []byte) error {
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

func (state *runnerState) verifyLocalForwardedAccess(ctx context.Context, admin personFixture, tailnet Remote) error {
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := admin.Remote.WaitReady(waitCtx); err != nil {
		return fmt.Errorf("verify local-forwarded access after Tailscale: %w", err)
	}
	if err := admin.Remote.Sudo(ctx, admin.LinuxPassword, nativeServiceChecks, "iso/local-forwarded-after-tailscale"); err != nil {
		return err
	}
	return state.checks.record("tailnet-access", verifyTailnetAfterLocalAccess(ctx, tailnet, admin.LinuxPassword))
}

func verifyTailnetAfterLocalAccess(ctx context.Context, tailnet Remote, password []byte) error {
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := tailnet.WaitReady(waitCtx); err != nil {
		return fmt.Errorf("verify Tailnet after local-forwarded access: %w", err)
	}
	if err := tailnet.Sudo(ctx, password, tailscaleAccessCheck, "iso/tailnet-after-local-forwarded"); err != nil {
		return err
	}
	output, err := CommandOutput(ctx, CommandSpec{Name: "curl", Args: []string{
		"--fail", "--silent", "--show-error", "--max-time", "10",
		"http://" + urlHost(tailnet.Host) + ":30000/api/healthz",
	}})
	if err != nil {
		return fmt.Errorf("verify Forgejo over Tailnet after local-forwarded access: %w", err)
	}
	return tailnet.Evidence.Write("iso/tailnet-forgejo-after-local-forwarded.txt", output)
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
		SSHPort: state.options.Ports.SSH, CockpitPort: state.options.Ports.Cockpit, ForgejoPort: state.options.Ports.Forgejo,
	}
	vm, err := LaunchVM(ctx, config)
	if err != nil {
		return nil, err
	}
	if err = state.registerVMCleanup(relative, vm); err != nil {
		stopCtx, cancel := StopDeadline()
		defer cancel()
		return nil, errors.Join(err, vm.Stop(stopCtx))
	}
	return vm, nil
}

func (state *runnerState) registerVMCleanup(relative string, vm *VM) error {
	if err := state.cleanup.Add(CleanupAction{Name: "QEMU " + relative, Run: vm.Stop}); err != nil {
		return err
	}
	if state.logout == nil {
		return nil
	}
	return state.cleanup.Add(CleanupAction{Name: "guest Tailnet enrollment before QEMU " + relative, Run: state.logout})
}

func (state *runnerState) resolveGuest(ctx context.Context, before tailnetStatus, tailnet Tailnet) (string, []byte, error) {
	discoveryCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	return tailnet.Discover(discoveryCtx, before)
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

func (state *runnerState) exerciseReusableQCOW2(ctx context.Context, inputs runInputs) error {
	if err := cloneDisk(ctx, state.artifacts.CandidateQCOW2, state.paths.qcowDisk); err != nil {
		return err
	}
	originalSize, err := state.growQCOW2(ctx)
	if err != nil {
		return err
	}
	seed, err := prepareQCOW2UserData(ctx, inputs.Admin, state.paths.work)
	if err != nil {
		return err
	}
	vm, err := state.launch(ctx, "qcow2/first-boot", "qcow2", state.paths.qcowDisk, seed)
	if err != nil {
		return err
	}
	fmt.Fprintf(state.output, "Cloud-init is provisioning reusable QCOW2 administrator %q using the protected password at %s and public key at %s. The disposable guest keeps Fedora firewall defaults with Cockpit TCP 9090 allowed; the suite administrator will open Forgejo ports for testing.\n", state.options.Administrator.Username, state.paths.password, state.paths.adminPublicKey)
	admin := inputs.Admin
	admin.Remote.KnownHosts = filepath.Join(state.paths.work, "qcow-known-hosts")
	remote := admin.Remote
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err = remote.WaitReady(waitCtx); err != nil {
		return err
	}
	if err = remote.Sudo(ctx, admin.LinuxPassword, acceptanceForgejoFirewall, "qcow2/administrator-allows-forgejo"); err != nil {
		return err
	}
	if err = state.verifyNativeOwner(ctx, admin, fmt.Sprintf("http://127.0.0.1:%d", state.options.Ports.Forgejo), inputs.OwnerPasswordFile); err != nil {
		return err
	}
	if err = runQCOW2Checks(ctx, admin, originalSize); err != nil {
		return err
	}
	if err = state.captureQMP(ctx, vm, "qcow2/qmp-running.json"); err != nil {
		return err
	}
	return vm.PowerDown(ctx)
}

type qemuImageInfo struct {
	VirtualSize int64 `json:"virtual-size"`
}

func (state *runnerState) growQCOW2(ctx context.Context) (int64, error) {
	before, raw, err := inspectQEMUImage(ctx, state.paths.qcowDisk)
	if err != nil {
		return 0, err
	}
	if err = state.evidence.Write("qcow2/image-before-growth.json", raw); err != nil {
		return 0, err
	}
	if err = RunCommand(ctx, CommandSpec{Name: "qemu-img", Args: []string{"resize", "--", state.paths.qcowDisk, state.options.DiskSize}}); err != nil {
		return 0, fmt.Errorf("grow reusable QCOW2: %w", err)
	}
	after, raw, err := inspectQEMUImage(ctx, state.paths.qcowDisk)
	if err != nil {
		return 0, err
	}
	if after.VirtualSize <= before.VirtualSize {
		return 0, fmt.Errorf("reusable QCOW2 did not grow beyond %d bytes", before.VirtualSize)
	}
	if err = state.evidence.Write("qcow2/image-after-growth.json", raw); err != nil {
		return 0, err
	}
	return before.VirtualSize, nil
}

func inspectQEMUImage(ctx context.Context, path string) (qemuImageInfo, []byte, error) {
	raw, err := CommandOutput(ctx, CommandSpec{Name: "qemu-img", Args: []string{"info", "--output=json", "--", path}})
	if err != nil {
		return qemuImageInfo{}, nil, err
	}
	var info qemuImageInfo
	if err = json.Unmarshal(raw, &info); err != nil || info.VirtualSize <= 0 {
		return qemuImageInfo{}, nil, errors.New("qemu-img returned an invalid virtual size")
	}
	return info, raw, nil
}

func cloneDisk(ctx context.Context, source, destination string) error {
	if runtime.GOOS == "linux" {
		return RunCommand(ctx, CommandSpec{Name: "cp", Args: []string{"--reflink=auto", "--", source, destination}})
	}
	return copyFile(source, destination)
}
