package acceptance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type VMConfig struct {
	Architecture string
	Mode         string
	Disk         string
	ISO          string
	Directory    string
	DiskSize     string
	Host         string
	SSHPort      int
	CockpitPort  int
}

type VM struct {
	Config  VMConfig
	Process *Process
	QMP     QMPClient
	outputs []io.Closer
}

func LaunchVM(ctx context.Context, config VMConfig) (*VM, error) {
	if err := prepareVMDisk(ctx, config); err != nil {
		return nil, err
	}
	serial, err := newPrivateFile(filepath.Join(config.Directory, "serial.log"))
	if err != nil {
		return nil, err
	}
	if err = serial.Close(); err != nil {
		return nil, err
	}
	args, binary, err := qemuCommand(config)
	if err != nil {
		return nil, err
	}
	stdout, err := newPrivateFile(filepath.Join(config.Directory, "qemu.stdout"))
	if err != nil {
		return nil, err
	}
	stderr, err := newPrivateFile(filepath.Join(config.Directory, "qemu.stderr"))
	if err != nil {
		stdout.Close()
		return nil, err
	}
	process, err := StartProcess(ctx, ProcessSpec{Name: binary, Args: args, Stdout: stdout, Stderr: stderr})
	if err != nil {
		stdout.Close()
		stderr.Close()
		return nil, err
	}
	if err = writeQEMUCommand(config.Directory, binary, args); err != nil {
		stopCtx, cancel := StopDeadline()
		defer cancel()
		return nil, errors.Join(err, process.Stop(stopCtx), stdout.Close(), stderr.Close())
	}
	vm := &VM{Config: config, Process: process, QMP: QMPClient{Socket: qmpSocket(config)}, outputs: []io.Closer{stdout, stderr}}
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = WaitForSocket(waitCtx, qmpSocket(config)); err != nil {
		stopCtx, cancel := StopDeadline()
		defer cancel()
		return nil, errors.Join(err, process.Stop(stopCtx), vm.closeOutputs())
	}
	return vm, nil
}

func writeQEMUCommand(directory, binary string, args []string) error {
	contents, err := json.MarshalIndent(map[string]any{"executable": binary, "arguments": args}, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return os.WriteFile(filepath.Join(directory, "qemu-command.json"), contents, 0o600)
}

func prepareVMDisk(ctx context.Context, config VMConfig) error {
	if config.Mode == "install" {
		if _, err := os.Lstat(config.Disk); err == nil {
			return fmt.Errorf("refusing to install over existing disk %s", config.Disk)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return RunCommand(ctx, CommandSpec{Name: "qemu-img", Args: []string{"create", "-f", "qcow2", config.Disk, config.DiskSize}})
	}
	return requireRegularFile(config.Disk)
}

func (vm *VM) PowerDown(ctx context.Context) error {
	if err := vm.QMP.Execute(ctx, "system_powerdown", "powerdown", nil, nil); err != nil {
		return err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	waitErr := vm.Process.Wait(waitCtx)
	return errors.Join(waitErr, vm.closeIO())
}

func (vm *VM) Stop(ctx context.Context) error {
	return errors.Join(vm.Process.Stop(ctx), vm.closeOutputs())
}

func (vm *VM) closeIO() error {
	return vm.closeOutputs()
}

func (vm *VM) closeOutputs() error {
	var result error
	for _, output := range vm.outputs {
		result = errors.Join(result, output.Close())
	}
	vm.outputs = nil
	return result
}

func qemuCommand(config VMConfig) ([]string, string, error) {
	switch config.Architecture {
	case "x86_64":
		return qemuX86Command(config)
	case "aarch64":
		return qemuARMCommand(config)
	default:
		return nil, "", fmt.Errorf("unsupported VM architecture %s", config.Architecture)
	}
}

func qmpSocket(config VMConfig) string { return filepath.Join(config.Directory, "qmp.sock") }

func WaitForSocket(ctx context.Context, path string) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			return nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for socket %s: %w", path, ctx.Err())
		case <-ticker.C:
		}
	}
}

func qemuX86Command(config VMConfig) ([]string, string, error) {
	binary := environmentOr("SODA_QEMU", "/usr/libexec/qemu-kvm")
	firmware := environmentOr("SODA_QEMU_FIRMWARE", "/usr/share/edk2/ovmf/OVMF_CODE.fd")
	varsTemplate := environmentOr("SODA_QEMU_VARS", "/usr/share/edk2/ovmf/OVMF_VARS.fd")
	vars := config.Disk + ".OVMF_VARS.fd"
	if config.Mode == "installed" {
		if err := requireRegularFile(vars); err != nil {
			return nil, "", fmt.Errorf("reuse OVMF variables: %w", err)
		}
	} else if err := copyFile(varsTemplate, vars); err != nil {
		return nil, "", fmt.Errorf("prepare OVMF variables: %w", err)
	}
	args := []string{"-machine", "q35,accel=kvm", "-cpu", "host", "-smp", "4", "-m", "8192"}
	args = append(args, "-drive", "if=pflash,format=raw,readonly=on,file="+firmware, "-drive", "if=pflash,format=raw,file="+vars)
	args = append(args, qemuCommonArgs(config)...)
	return args, binary, nil
}

func qemuARMCommand(config VMConfig) ([]string, string, error) {
	binary := environmentOr("SODA_QEMU", "qemu-system-aarch64")
	firmware := environmentOr("SODA_QEMU_FIRMWARE", armFirmware())
	acceleration := map[string]string{"darwin": "hvf", "linux": "kvm"}[runtime.GOOS]
	if acceleration == "" {
		return nil, "", fmt.Errorf("AArch64 QEMU is unsupported on %s", runtime.GOOS)
	}
	args := []string{"-machine", "virt,accel=" + acceleration, "-cpu", "host", "-smp", "4", "-m", "8192", "-bios", firmware}
	args = append(args, qemuCommonArgs(config)...)
	return args, binary, nil
}

func qemuCommonArgs(config VMConfig) []string {
	args := []string{"-drive", "file=" + config.Disk + ",if=virtio,format=qcow2"}
	if config.Mode == "install" {
		cdrom := "file=" + config.ISO + ",media=cdrom,format=raw,readonly=on"
		if config.Architecture == "aarch64" {
			cdrom += ",if=virtio"
		}
		args = append(args, "-drive", cdrom, "-boot", "order=c,once=d")
	} else {
		args = append(args, "-boot", "order=c")
	}
	network := fmt.Sprintf("user,id=net0,hostfwd=tcp:%s:%d-:22,hostfwd=tcp:%s:%d-:9090,hostfwd=tcp:%s:18080-:18080,hostfwd=tcp:%s:18081-:18081", config.Host, config.SSHPort, config.Host, config.CockpitPort, config.Host, config.Host)
	args = append(args, "-netdev", network, "-device", "virtio-net-pci,netdev=net0")
	args = append(args, "-serial", "file:"+filepath.Join(config.Directory, "serial.log"), "-monitor", "none", "-qmp", "unix:"+qmpSocket(config)+",server=on,wait=off")
	return append(args, qemuDisplayArgs(config)...)
}

func qemuDisplayArgs(config VMConfig) []string {
	if config.Mode == "installed" {
		return []string{"-display", "none"}
	}
	if runtime.GOOS == "darwin" {
		return []string{"-device", "virtio-gpu-pci", "-display", "cocoa", "-device", "qemu-xhci", "-device", "usb-kbd", "-device", "usb-tablet"}
	}
	if config.Architecture == "aarch64" {
		return []string{"-device", "virtio-gpu-pci", "-display", "gtk", "-device", "qemu-xhci", "-device", "usb-kbd", "-device", "usb-tablet"}
	}
	return []string{"-device", "virtio-vga", "-display", "gtk"}
}

func armFirmware() string {
	if runtime.GOOS == "darwin" {
		return "/opt/homebrew/share/qemu/edk2-aarch64-code.fd"
	}
	return "/usr/share/edk2/aarch64/QEMU_EFI.fd"
}

func environmentOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	return errors.Join(copyErr, closeErr)
}
