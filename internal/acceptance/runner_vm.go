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

func (state *runnerState) installAndOnboard(ctx context.Context) (scenarioState, *VM, error) {
	tailnet, err := NewTailnet()
	if err != nil {
		return scenarioState{}, nil, err
	}
	state.tailnet = tailnet
	before, raw, err := tailnet.Snapshot(ctx)
	if err != nil {
		return scenarioState{}, nil, err
	}
	if err = state.evidence.Write("iso/host-tailnet-before.json", raw); err != nil {
		return scenarioState{}, nil, err
	}
	return state.completeISOFlow(ctx, before)
}

func (state *runnerState) completeISOFlow(ctx context.Context, before tailnetStatus) (scenarioState, *VM, error) {
	vm, err := state.launch(ctx, "iso/install", "install", state.paths.installedDisk, state.artifacts.CandidateISO)
	if err != nil {
		return scenarioState{}, nil, err
	}
	fmt.Fprintf(state.output, "Create Linux administrator %q through graphical Anaconda, reboot, and log in normally. Add the personal public key through Cockpit Accounts before continuing. Protected input paths:\n  password: %s\n  SSH public key: %s\n\n", state.options.Administrator.Username, state.paths.password, state.paths.adminPublicKey)
	password, err := os.ReadFile(state.paths.password)
	if err != nil {
		return scenarioState{}, vm, err
	}
	if err = state.verifyInitialLAN(ctx, password); err != nil {
		return scenarioState{}, vm, err
	}
	fmt.Fprintln(state.output, "LAN access is verified. Open Cockpit → Tailscale and sign in through its native browser authentication URL.")
	host, raw, err := state.resolveGuest(ctx, before)
	if err != nil {
		return scenarioState{}, vm, err
	}
	if err = state.evidence.Write("iso/host-tailnet-enrolled.json", raw); err != nil {
		return scenarioState{}, vm, err
	}
	if err = state.evidence.Write("iso/tailnet-address.txt", []byte(host+"\n")); err != nil {
		return scenarioState{}, vm, err
	}
	remote := state.tailnetRemote(host)
	if err = state.registerTailnetCleanup(&remote, password); err != nil {
		return scenarioState{}, vm, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err = remote.WaitReady(waitCtx); err != nil {
		return scenarioState{}, vm, err
	}
	remote, vm, err = state.completeInstalledAccess(ctx, remote, vm, password)
	return scenarioState{remote: remote, tailnetHost: host}, vm, err
}

func (state *runnerState) completeInstalledAccess(ctx context.Context, remote Remote, vm *VM, password []byte) (Remote, *VM, error) {
	local, err := state.verifyLAN(ctx, remote, password)
	if err != nil {
		return Remote{}, vm, err
	}
	return local, vm, state.captureQMP(ctx, vm, "iso/qmp-running.json")
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

func (state *runnerState) verifyLAN(ctx context.Context, tailnet Remote, password []byte) (Remote, error) {
	local := state.localRemote()
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := local.WaitReady(waitCtx); err != nil {
		return Remote{}, fmt.Errorf("verify LAN after Tailscale: %w", err)
	}
	if err := local.Sudo(ctx, password, localAccessCheck, "iso/lan-after-tailscale"); err != nil {
		return Remote{}, err
	}
	if err := state.verifyTailnetAfterLAN(ctx, tailnet); err != nil {
		return Remote{}, err
	}
	return local, nil
}

func (state *runnerState) verifyTailnetAfterLAN(ctx context.Context, tailnet Remote) error {
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := tailnet.WaitReady(waitCtx); err != nil {
		return fmt.Errorf("verify Tailnet after LAN: %w", err)
	}
	if err := tailnet.Sudo(ctx, state.secret("administrator-password"), tailscaleAccessCheck, "iso/tailnet-after-lan"); err != nil {
		return err
	}
	output, err := CommandOutput(ctx, CommandSpec{Name: "curl", Args: []string{
		"--fail", "--silent", "--show-error", "--max-time", "10",
		"http://" + urlHost(tailnet.Host) + ":30000/api/healthz",
	}})
	if err != nil {
		return fmt.Errorf("verify Forgejo over Tailnet after LAN: %w", err)
	}
	return state.evidence.Write("iso/tailnet-forgejo-after-lan.txt", output)
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
	originalSize, err := state.growQCOW2(ctx)
	if err != nil {
		return err
	}
	seed, err := state.prepareQCOW2UserData(ctx)
	if err != nil {
		return err
	}
	vm, err := state.launch(ctx, "qcow2/first-boot", "qcow2", state.paths.qcowDisk, seed)
	if err != nil {
		return err
	}
	fmt.Fprintf(state.output, "Cloud-init is provisioning reusable QCOW2 administrator %q using the protected password at %s and public key at %s. The disposable guest uses ordinary LAN access with firewalld disabled by default.\n", state.options.Administrator.Username, state.paths.password, state.paths.adminPublicKey)
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
	if err = state.verifyNativeOwner(ctx, remote, fmt.Sprintf("http://127.0.0.1:%d", state.options.Ports.Forgejo)); err != nil {
		return err
	}
	if err = state.runQCOW2Checks(ctx, remote, originalSize); err != nil {
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
