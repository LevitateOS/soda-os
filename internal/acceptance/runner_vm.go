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

func (state *runnerState) installAndOnboard(ctx context.Context, inputs runInputs) (string, *guest, error) {
	installed, err := state.launch(ctx, "iso/install", "install", state.paths.installedDisk, state.artifacts.CandidateISO)
	if err != nil {
		return "", nil, err
	}
	admin := inputs.Admin
	fmt.Fprintf(state.output, "Create Linux administrator %q through graphical Anaconda, reboot, and log in normally. Add the personal public key through Cockpit Accounts before continuing. Protected input paths:\n  password: %s\n  SSH public key: %s\n\n", admin.Remote.Username, inputs.PasswordFile, inputs.PublicKeyFile)
	if err = state.checks.record("iso-first-boot-defaults", verifyInitialLocalForwardedAccess(ctx, admin, state.options.Ports.Forgejo)); err != nil {
		return "", installed, err
	}
	fmt.Fprintln(state.output, "Local-forwarded access through QEMU is verified (not independent LAN evidence). Open Cockpit → Tailscale and sign in through its native browser authentication URL.")
	host, raw, err := awaitGuestEnrollment(ctx, installed, admin)
	if err != nil {
		return "", installed, err
	}
	remote := admin.Remote
	remote.Host, remote.Port, remote.CockpitPort = host, 22, 9090
	remote.KnownHosts = state.paths.knownHosts
	if err = state.evidence.Write("iso/guest-tailnet-enrolled.json", raw); err != nil {
		return "", installed, err
	}
	if err = state.evidence.Write("iso/tailnet-address.txt", []byte(host+"\n")); err != nil {
		return "", installed, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err = remote.WaitReady(waitCtx); err != nil {
		return "", installed, err
	}
	if err = state.checks.record("local-forwarded-access", verifyLocalForwardedAccess(ctx, admin, remote, state.checks)); err != nil {
		return "", installed, err
	}
	return host, installed, installed.captureQMP(ctx, "iso/qmp-running.json")
}

func verifyLocalForwardedAccess(ctx context.Context, admin personFixture, tailnet Remote, checks checkResults) error {
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := admin.Remote.WaitReady(waitCtx); err != nil {
		return fmt.Errorf("verify local-forwarded access after Tailscale: %w", err)
	}
	if err := admin.Remote.Sudo(ctx, admin.LinuxPassword, nativeServiceChecks, "iso/local-forwarded-after-tailscale"); err != nil {
		return err
	}
	return checks.record("tailnet-access", verifyTailnetAfterLocalAccess(ctx, tailnet, admin.LinuxPassword))
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

func (state *runnerState) launch(ctx context.Context, relative, mode, disk, iso string) (*guest, error) {
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
	return launchGuest(ctx, config, state.evidence, state.cleanup)
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
	qcow, err := state.launch(ctx, "qcow2/first-boot", "qcow2", state.paths.qcowDisk, seed)
	if err != nil {
		return err
	}
	fmt.Fprintf(state.output, "Cloud-init is provisioning reusable QCOW2 administrator %q using the protected password at %s and public key at %s. The disposable guest keeps Fedora firewall defaults with Cockpit TCP 9090 allowed; the suite administrator will open Forgejo ports for testing.\n", inputs.Admin.Remote.Username, inputs.PasswordFile, inputs.PublicKeyFile)
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
	if err = awaitNativeOwnerSignup(ctx, admin, fmt.Sprintf("http://127.0.0.1:%d", state.options.Ports.Forgejo), inputs.OwnerPasswordFile, state.output); err != nil {
		return err
	}
	if err = runQCOW2Checks(ctx, admin, originalSize); err != nil {
		return err
	}
	if err = qcow.captureQMP(ctx, "qcow2/qmp-running.json"); err != nil {
		return err
	}
	return qcow.shutdown(ctx)
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
