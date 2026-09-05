package scripts

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func preparePlacementSource(t *testing.T, fixture nativeScriptFixture) string {
	t.Helper()
	path := filepath.Join(fixture.root, "SodaOS-0.6.3-"+fixture.arch+".iso")
	writeTestFile(t, path, "iso", 0o644)
	sidecar := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte("iso")), filepath.Base(path))
	writeTestFile(t, path+".sha256", sidecar, 0o644)
	return path
}

func TestPlaceLibvirtISOBothArchitectures(t *testing.T) {
	for _, arch := range []string{"aarch64", "x86_64"} {
		t.Run(arch, func(t *testing.T) {
			fixture := prepareNativeScriptTest(t, arch, "Linux")
			source := preparePlacementSource(t, fixture)
			directory := filepath.Join(fixture.root, "destination")
			runScriptOK(t, fixture.command(t, "place-libvirt-iso.sh", arch, source, directory))
			commands := readFile(t, fixture.log)
			requireOrder(t, commands, "sha256sum "+source, "sudo -n -u qemu -- test -x "+directory,
				"sha256sum "+directory, "sudo -n -u qemu -- test -r", "stat -c %C", "sudo -n -u qemu -- sh -eu -c")
			require.Contains(t, commands, "qemu-system-"+arch)
			for _, forbidden := range []string{"/home/libvirt", "docker ", "just ", "skopeo ", "go run", "git "} {
				require.NotContains(t, commands, forbidden)
			}
			require.Equal(t, "iso", readFile(t, filepath.Join(directory, filepath.Base(source))))
		})
	}
}

func TestPlaceLibvirtISORefusesOverwrite(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
		source := preparePlacementSource(t, fixture)
		directory := filepath.Join(fixture.root, "destination")
		destination := filepath.Join(directory, filepath.Base(source))
		if symlink {
			if err := os.Symlink("missing-target", destination); err != nil {
				t.Fatal(err)
			}
		} else {
			writeTestFile(t, destination, "keep", 0o644)
		}
		runScriptFails(t, fixture.command(t, "place-libvirt-iso.sh", fixture.arch, source, directory), "already exists")
		if !symlink && readFile(t, destination) != "keep" {
			t.Fatal("existing destination changed")
		}
	}
}

func TestPlaceLibvirtISOVerifiesSourceBeforeCopy(t *testing.T) {
	fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
	source := preparePlacementSource(t, fixture)
	writeTestFile(t, source, "changed", 0o644)
	directory := filepath.Join(fixture.root, "destination")
	runScriptFails(t, fixture.command(t, "place-libvirt-iso.sh", fixture.arch, source, directory), "sidecar is missing or stale")
	if _, err := os.Lstat(filepath.Join(directory, filepath.Base(source))); !os.IsNotExist(err) {
		t.Fatal("unverified source was copied")
	}
}

func TestPlaceLibvirtISOFailureBoundaries(t *testing.T) {
	for _, failure := range []string{"traversal", "copy-checksum", "readable", "label", "qemu-open"} {
		t.Run(failure, func(t *testing.T) {
			fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
			source := preparePlacementSource(t, fixture)
			directory := filepath.Join(fixture.root, "destination")
			cmd := fixture.command(t, "place-libvirt-iso.sh", fixture.arch, source, directory)
			cmd.Env = append(cmd.Env, "SODA_TEST_FAIL="+failure)
			output := runScriptFails(t, cmd, "")
			destination := filepath.Join(directory, filepath.Base(source))
			_, err := os.Lstat(destination)
			if failure == "traversal" {
				require.True(t, os.IsNotExist(err), "copy started without destination traversal")
				return
			}
			require.NoError(t, err)
			require.Contains(t, output, "destination may be partial or complete: "+destination)
		})
	}
}

func TestPlaceLibvirtISORejectsDarwin(t *testing.T) {
	fixture := prepareNativeScriptTest(t, "aarch64", "Darwin")
	runScriptFails(t, fixture.command(t, "place-libvirt-iso.sh", fixture.arch, "unused.iso", "/unused"), "requires matching native Linux")
}
