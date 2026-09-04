package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LevitateOS/soda-os/internal/process"
)

type isoInspectionInput struct {
	lock                                          toolLock
	volumeName, installerTag, isoPath, inspectDir string
	reference                                     string
}

const anacondaBootcInstallationPath = "usr/lib64/python3.14/site-packages/pyanaconda/modules/payloads/payload/rpm_ostree/installation.py"

func (b *Builder) inspectISO(ctx context.Context, input isoInspectionInput) error {
	outer := []string{"run", "--rm", "--platform", b.Spec.Base.Platform, "--privileged", "--entrypoint", "podman", "--volume", input.volumeName + ":/var/lib/containers/storage", "--volume", input.isoPath + ":/input/soda.iso:ro", "--volume", input.inspectDir + ":/inspect", input.lock.Reference, "run", "--rm", "--privileged", "--security-opt", "label=disable", "--volume", "/input/soda.iso:/input/soda.iso:ro", "--volume", "/inspect:/inspect", input.installerTag}
	args := append(append([]string{}, outer...), "xorriso", "-osirrox", "on", "-indev", "/input/soda.iso", "-extract", "/LiveOS/squashfs.img", "/inspect/squashfs.img")
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("extract installer squashfs: %w", err)
	}
	listing, err := b.runner.Output(ctx, process.Command{Dir: b.Root, Name: "docker", Args: append(append([]string{}, outer...), "unsquashfs", "-ll", "/inspect/squashfs.img")})
	if err != nil {
		return fmt.Errorf("list installer squashfs: %w", err)
	}
	if err := validateNoDuplicatedBootcBase(listing); err != nil {
		return err
	}
	args = append(append([]string{}, outer...), "xorriso", "-osirrox", "on", "-indev", "/input/soda.iso", "-extract", "/images/pxeboot/initrd.img", "/inspect/initrd.img")
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("extract installer initramfs: %w", err)
	}
	initramfs, err := b.runner.Output(ctx, process.Command{Dir: b.Root, Name: "docker", Args: append(append([]string{}, outer...), "lsinitrd", "/inspect/initrd.img")})
	if err != nil {
		return fmt.Errorf("list installer initramfs: %w", err)
	}
	if err := validateInstallerInitramfs(initramfs); err != nil {
		return err
	}
	args = append(append([]string{}, outer...), "unsquashfs", "-f", "-d", "/inspect/root", "/inspect/squashfs.img", ".buildstamp", "usr/lib/os-release", "usr/share/anaconda/interactive-defaults.ks", "etc/anaconda/conf.d/90-soda-storage.conf", "etc/anaconda/profile.d/sodaos.conf", "etc/systemd/system/anaconda.target.wants/var-tmp.mount", "usr/share/anaconda/pixmaps/soda.css", "usr/share/anaconda/pixmaps/soda-sidebar-logo.png", "usr/share/anaconda/pixmaps/soda-symbol.png", "usr/lib/image-builder/bootc/iso.yaml", "usr/lib/systemd/system/var-tmp.mount", anacondaBootcInstallationPath, "usr/share/anaconda/addons", "usr/share/anaconda/dbus/confs", "usr/share/anaconda/dbus/services", "var/lib/containers/storage/overlay-images/images.json")
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("inspect installer squashfs: %w", err)
	}
	args = []string{"run", "--rm", "--platform", b.Spec.Base.Platform, "--volume", input.inspectDir + ":/inspect", "--entrypoint", "chown", input.lock.Reference, "-R", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "/inspect"}
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("own extracted ISO inspection files: %w", err)
	}
	if err := makeInspectionOwnerWritable(input.inspectDir); err != nil {
		return fmt.Errorf("make extracted ISO inspection files readable: %w", err)
	}
	return b.validateExtractedISO(input.inspectDir, input.reference)
}

func validateNoDuplicatedBootcBase(listing string) error {
	if strings.Contains(listing, "squashfs-root/sysroot") {
		return errors.New("installer squashfs contains a duplicated bootc base")
	}
	return nil
}

func validateInstallerInitramfs(listing string) error {
	if !strings.Contains(listing, "usr/share/anaconda/interactive-defaults.ks") {
		return errors.New("installer initramfs lacks the interactive defaults")
	}
	return nil
}

func makeInspectionOwnerWritable(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		switch {
		case entry.IsDir():
			mode |= 0o700
		case mode.IsRegular():
			mode |= 0o600
		default:
			return nil
		}
		return os.Chmod(path, mode)
	})
}

func (b *Builder) validateExtractedISO(inspectDir, reference string) error {
	if err := b.validateExtractedKickstart(inspectDir, reference); err != nil {
		return err
	}
	if err := validateExtractedPayload(inspectDir, reference); err != nil {
		return err
	}
	if err := b.validateExtractedConfiguration(inspectDir); err != nil {
		return err
	}
	if err := b.validateExtractedInstallerScratch(inspectDir); err != nil {
		return err
	}
	if err := validateNoInstallerProvisioning(inspectDir); err != nil {
		return err
	}
	return b.validateExtractedBranding(inspectDir)
}

func (b *Builder) validateExtractedKickstart(inspectDir, reference string) error {
	actual, err := os.ReadFile(filepath.Join(inspectDir, "root", "usr/share/anaconda/interactive-defaults.ks"))
	if err != nil {
		return fmt.Errorf("read ISO kickstart: %w", err)
	}
	if string(actual) != kickstart(reference, b.Spec.Identity.Hostname) {
		return errors.New("ISO kickstart differs from exact Soda payload contract")
	}
	return nil
}

func validateExtractedPayload(inspectDir, reference string) error {
	metadata, err := os.ReadFile(filepath.Join(inspectDir, "root", "var/lib/containers/storage/overlay-images/images.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read ISO container storage metadata: %w", err)
	}
	return validateNoEmbeddedPayload(metadata, reference)
}

func (b *Builder) validateExtractedConfiguration(inspectDir string) error {
	for _, file := range []struct {
		actual, expected, label, mismatch string
	}{{"usr/lib/image-builder/bootc/iso.yaml", b.Spec.Platform.Installer.ISOConfig, "ISO configuration", "ISO boot configuration differs from the Soda installer contract"}, {"etc/anaconda/conf.d/90-soda-storage.conf", filepath.Join("packaging", "installer", "soda-storage.conf"), "installer storage configuration", "ISO storage configuration differs from the Soda ext4 root-only contract"}} {
		actual, err := os.ReadFile(filepath.Join(inspectDir, "root", file.actual))
		if err != nil {
			return fmt.Errorf("read %s: %w", file.label, err)
		}
		expectedPath := file.expected
		if !filepath.IsAbs(expectedPath) {
			expectedPath = filepath.Join(b.Root, expectedPath)
		}
		expected, err := os.ReadFile(expectedPath)
		if err != nil {
			return fmt.Errorf("read expected %s: %w", file.label, err)
		}
		if !bytes.Equal(actual, expected) {
			return errors.New(file.mismatch)
		}
	}
	return nil
}

func (b *Builder) validateExtractedInstallerScratch(inspectDir string) error {
	actual, err := os.ReadFile(filepath.Join(inspectDir, "root", "usr/lib/systemd/system/var-tmp.mount"))
	if err != nil {
		return fmt.Errorf("read ISO installer scratch mount: %w", err)
	}
	expected, err := os.ReadFile(filepath.Join(b.Root, "packaging", "installer", "var-tmp.mount"))
	if err != nil {
		return fmt.Errorf("read expected installer scratch mount: %w", err)
	}
	if !bytes.Equal(actual, expected) {
		return errors.New("ISO installer scratch mount differs from the Soda installer contract")
	}
	wants := filepath.Join(inspectDir, "root", "etc/systemd/system/anaconda.target.wants/var-tmp.mount")
	target, err := os.Readlink(wants)
	if err != nil {
		return fmt.Errorf("read ISO installer scratch mount enablement: %w", err)
	}
	if target != "/usr/lib/systemd/system/var-tmp.mount" {
		return errors.New("ISO installer scratch mount is not enabled for Anaconda")
	}
	return nil
}

func validateNoInstallerProvisioning(inspectDir string) error {
	for _, obsolete := range []string{
		"usr/libexec/soda/soda-installer-input",
		"usr/libexec/soda/soda-installer-finalize",
		"usr/share/anaconda/addons/org_fedoraproject_soda",
		"usr/share/anaconda/dbus/confs/org.fedoraproject.Anaconda.Addons.SodaInstaller.conf",
		"usr/share/anaconda/dbus/services/org.fedoraproject.Anaconda.Addons.SodaInstaller.service",
	} {
		if _, err := os.Lstat(filepath.Join(inspectDir, "root", obsolete)); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("obsolete Soda installer provisioning path remains in ISO: %s", obsolete)
		}
	}
	anacondaBootc, err := os.ReadFile(filepath.Join(inspectDir, "root", anacondaBootcInstallationPath))
	if err != nil {
		return fmt.Errorf("read ISO Anaconda bootc mount implementation: %w", err)
	}
	if bytes.Count(anacondaBootc, []byte(`for path in ("/proc", "/sys", "/sys/fs/selinux"):`)) != 1 ||
		bytes.Contains(anacondaBootc, []byte(`for path in ("/proc", "/sys"):`)) {
		return errors.New("ISO Anaconda bootc mount implementation lacks the reviewed SELinuxFS correction")
	}
	return nil
}

func (b *Builder) validateExtractedBranding(inspectDir string) error {
	for _, file := range []struct{ actual, expected string }{
		{".buildstamp", "packaging/installer/branding/buildstamp"},
		{"usr/lib/os-release", "packaging/installer/branding/os-release"},
		{"etc/anaconda/profile.d/sodaos.conf", "packaging/installer/branding/sodaos.conf"},
		{"usr/share/anaconda/pixmaps/soda.css", "packaging/installer/branding/soda.css"},
		{"usr/share/anaconda/pixmaps/soda-sidebar-logo.png", "assets/branding/installer/soda-logo-horizontal-dark.png"},
		{"usr/share/anaconda/pixmaps/soda-symbol.png", "assets/branding/installer/soda-symbol.png"},
	} {
		actual, err := os.ReadFile(filepath.Join(inspectDir, "root", file.actual))
		if err != nil {
			return fmt.Errorf("read ISO Anaconda branding: %w", err)
		}
		expected, err := os.ReadFile(filepath.Join(b.Root, file.expected))
		if err != nil {
			return fmt.Errorf("read expected Anaconda branding: %w", err)
		}
		if !bytes.Equal(actual, expected) {
			return errors.New("ISO Anaconda branding differs from the Soda installer contract")
		}
	}
	return nil
}
