package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	imagebuild "github.com/LevitateOS/soda-os/internal/image"
)

type isoInspectionInput struct {
	lock                                          toolLock
	volumeName, installerTag, isoPath, inspectDir string
	reference, payloadTag                         string
}

func (b *Builder) inspectISO(ctx context.Context, input isoInspectionInput) error {
	outer := []string{"run", "--rm", "--platform", Platform, "--privileged", "--entrypoint", "podman", "--volume", input.volumeName + ":/var/lib/containers/storage", "--volume", input.isoPath + ":/input/soda.iso:ro", "--volume", input.inspectDir + ":/inspect", input.lock.Reference, "run", "--rm", "--privileged", "--security-opt", "label=disable", "--volume", "/input/soda.iso:/input/soda.iso:ro", "--volume", "/inspect:/inspect", input.installerTag}
	args := append(append([]string{}, outer...), "xorriso", "-osirrox", "on", "-indev", "/input/soda.iso", "-extract", "/LiveOS/squashfs.img", "/inspect/squashfs.img")
	if err := b.runner.Run(ctx, imagebuild.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("extract installer squashfs: %w", err)
	}
	args = append(append([]string{}, outer...), "unsquashfs", "-f", "-d", "/inspect/root", "/inspect/squashfs.img", "usr/share/anaconda/interactive-defaults.ks", "etc/anaconda/conf.d/90-soda-storage.conf", "usr/lib/image-builder/bootc/iso.yaml", "var/lib/containers/storage/overlay-images/images.json")
	if err := b.runner.Run(ctx, imagebuild.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("inspect installer squashfs: %w", err)
	}
	return b.validateExtractedISO(input.inspectDir, input.reference, input.payloadTag)
}

func (b *Builder) validateExtractedISO(inspectDir, reference, payloadTag string) error {
	if err := b.validateExtractedKickstart(inspectDir, reference); err != nil {
		return err
	}
	if err := validateExtractedPayload(inspectDir, payloadTag, reference); err != nil {
		return err
	}
	return b.validateExtractedConfiguration(inspectDir)
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

func validateExtractedPayload(inspectDir, payloadTag, reference string) error {
	metadata, err := os.ReadFile(filepath.Join(inspectDir, "root", "var/lib/containers/storage/overlay-images/images.json"))
	if err != nil {
		return fmt.Errorf("read embedded container storage metadata: %w", err)
	}
	return validateEmbeddedPayload(metadata, payloadTag, reference)
}

func (b *Builder) validateExtractedConfiguration(inspectDir string) error {
	for _, file := range []struct {
		actual, expected, label, mismatch string
	}{{"usr/lib/image-builder/bootc/iso.yaml", "iso.yaml", "ISO configuration", "ISO boot configuration differs from the Soda installer contract"}, {"etc/anaconda/conf.d/90-soda-storage.conf", "soda-storage.conf", "installer storage configuration", "ISO storage configuration differs from the Soda ext4 root-only contract"}} {
		actual, err := os.ReadFile(filepath.Join(inspectDir, "root", file.actual))
		if err != nil {
			return fmt.Errorf("read %s: %w", file.label, err)
		}
		expected, err := os.ReadFile(filepath.Join(b.Root, "packaging", "installer", file.expected))
		if err != nil {
			return fmt.Errorf("read expected %s: %w", file.label, err)
		}
		if !bytes.Equal(actual, expected) {
			return errors.New(file.mismatch)
		}
	}
	return nil
}
