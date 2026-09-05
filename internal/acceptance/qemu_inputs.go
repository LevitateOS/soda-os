package acceptance

import (
	"fmt"
	"os"
)

// These are the native executable and firmware inputs, shared by preflight and
// command construction. Resolving them neither creates disks nor copies firmware.
type qemuInputs struct {
	Binary    string
	Firmware  string
	Variables string
}

func qemuHostInputs(architecture string) (qemuInputs, error) {
	var inputs qemuInputs
	switch architecture {
	case "x86_64":
		inputs = qemuInputs{Binary: "/usr/libexec/qemu-kvm", Firmware: "/usr/share/edk2/ovmf/OVMF_CODE.fd", Variables: "/usr/share/edk2/ovmf/OVMF_VARS.fd"}
	case "aarch64":
		inputs = qemuInputs{Binary: "qemu-system-aarch64", Firmware: armFirmware()}
	default:
		return qemuInputs{}, fmt.Errorf("unsupported QEMU architecture %s", architecture)
	}
	inputs.Binary = environmentOr("SODA_QEMU", inputs.Binary)
	inputs.Firmware = environmentOr("SODA_QEMU_FIRMWARE", inputs.Firmware)
	if inputs.Variables != "" {
		inputs.Variables = environmentOr("SODA_QEMU_VARS", inputs.Variables)
	}
	return inputs, nil
}

func requireQEMUInputs() error {
	inputs, err := qemuHostInputs(nativeArchitecture())
	if err != nil {
		return err
	}
	if err = RequireCommands(inputs.Binary); err != nil {
		return err
	}
	for _, path := range []string{inputs.Firmware, inputs.Variables} {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("QEMU firmware %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("QEMU firmware %s is not a regular file", path)
		}
	}
	return nil
}
