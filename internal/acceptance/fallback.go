package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type bootcStatus struct {
	Status struct {
		Booted struct {
			Image struct {
				Digest string `json:"imageDigest"`
			} `json:"image"`
		} `json:"booted"`
	} `json:"status"`
}

func exerciseFallback(ctx context.Context, admin personFixture, guest *guest, candidate, fallback string) error {
	before, err := captureManifest(ctx, admin, "fallback/b-before")
	if err != nil {
		return err
	}
	registry, _, _ := strings.Cut(fallback, "/")
	if err := enableGuestRegistry(ctx, admin, registry); err != nil {
		return err
	}
	if err := switchImage(ctx, admin, guest, "fallback", fallback); err != nil {
		return err
	}
	selected, err := captureManifest(ctx, admin, "fallback/a-selected")
	if err != nil {
		return err
	}
	if err := compareManifests(before, selected, "fallback/a-selected"); err != nil {
		return err
	}
	if err := switchImage(ctx, admin, guest, "candidate", candidate); err != nil {
		return err
	}
	restored, err := captureManifest(ctx, admin, "fallback/b-restored")
	if err != nil {
		return err
	}
	if err := compareManifests(before, restored, "fallback/b-restored"); err != nil {
		return err
	}
	return disableGuestRegistry(ctx, admin)
}

func captureManifest(ctx context.Context, admin personFixture, relative string) ([]byte, error) {
	return admin.Remote.SudoOutput(ctx, admin.LinuxPassword, stableManifestScript, relative)
}

func compareManifests(expected, actual []byte, label string) error {
	if !bytes.Equal(expected, actual) {
		return fmt.Errorf("normalized preservation manifests differ: fallback/b-before and %s", label)
	}
	return nil
}

func enableGuestRegistry(ctx context.Context, admin personFixture, registry string) error {
	script := "install -d -m 0755 /etc/containers/registries.conf.d\n" +
		"printf '%s\\n' '[[registry]]' 'location = \"" + registry + "\"' 'insecure = true' > /etc/containers/registries.conf.d/99-soda-acceptance.conf\n" +
		"chmod 0644 /etc/containers/registries.conf.d/99-soda-acceptance.conf\n"
	return admin.Remote.Sudo(ctx, admin.LinuxPassword, script, "fallback/registry-enable")
}

func disableGuestRegistry(ctx context.Context, admin personFixture) error {
	script := "test -f /etc/containers/registries.conf.d/99-soda-acceptance.conf\nrm -- /etc/containers/registries.conf.d/99-soda-acceptance.conf\n"
	return admin.Remote.Sudo(ctx, admin.LinuxPassword, script, "fallback/registry-disable")
}

func switchImage(ctx context.Context, admin personFixture, guest *guest, target, reference string) error {
	_, digest, _ := strings.Cut(reference, "@")
	stageInput := append(bytes.TrimRight(admin.LinuxPassword, "\r\n"), '\n')
	_, err := admin.Remote.CaptureOutput(ctx, "fallback/"+target+"-download", stageInput,
		"sudo", "-k", "-S", "-p", "", "/usr/bin/bootc", "switch", "--download-only", reference)
	if err != nil {
		return err
	}
	_, err = admin.Remote.CaptureOutput(ctx, "fallback/"+target+"-activate", stageInput,
		"sudo", "-k", "-S", "-p", "", "/usr/bin/bootc", "switch", "--from-downloaded")
	if err != nil {
		return err
	}
	if err = guest.restart(ctx, "fallback/boot-"+target); err != nil {
		return err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err = admin.Remote.WaitReady(waitCtx); err != nil {
		return err
	}
	return assertBootedDigest(ctx, admin, target, digest)
}

func assertBootedDigest(ctx context.Context, admin personFixture, target, digest string) error {
	input := append(bytes.TrimRight(admin.LinuxPassword, "\r\n"), '\n')
	output, err := admin.Remote.CaptureOutput(ctx, "fallback/"+target+"-bootc-status", input,
		"sudo", "-k", "-S", "-p", "", "/usr/bin/bootc", "status", "--format=json")
	if err != nil {
		return err
	}
	var status bootcStatus
	if err = json.Unmarshal(output, &status); err != nil {
		return err
	}
	if status.Status.Booted.Image.Digest != digest {
		return fmt.Errorf("booted digest %s does not match %s", status.Status.Booted.Image.Digest, digest)
	}
	return nil
}

// The port and release record have already passed input validation.
func guestImageReference(port int, image releaseRecord) string {
	return "10.0.2.2:" + strconv.Itoa(port) + "/soda-os@" + imageDigest(image)
}
