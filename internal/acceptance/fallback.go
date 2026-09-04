package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

func (state *runnerState) exerciseFallback(ctx context.Context, scenario *scenarioState, vm **VM) error {
	if err := state.captureManifest(ctx, scenario.remote, "fallback/b-before"); err != nil {
		return err
	}
	if err := state.enableGuestRegistry(ctx, *scenario); err != nil {
		return err
	}
	if err := state.switchImage(ctx, scenario, vm, "fallback"); err != nil {
		return err
	}
	if err := state.captureManifest(ctx, scenario.remote, "fallback/a-selected"); err != nil {
		return err
	}
	if err := state.compareManifests("fallback/b-before", "fallback/a-selected"); err != nil {
		return err
	}
	if err := state.switchImage(ctx, scenario, vm, "candidate"); err != nil {
		return err
	}
	if err := state.captureManifest(ctx, scenario.remote, "fallback/b-restored"); err != nil {
		return err
	}
	if err := state.compareManifests("fallback/b-before", "fallback/b-restored"); err != nil {
		return err
	}
	return state.disableGuestRegistry(ctx, *scenario)
}

func (state *runnerState) captureManifest(ctx context.Context, remote Remote, relative string) error {
	return remote.Sudo(ctx, state.secret("administrator-password"), stableManifestScript, relative)
}

func (state *runnerState) compareManifests(expected, actual string) error {
	expectedPath := filepath.Join(state.evidence.Root, expected+".stdout")
	actualPath := filepath.Join(state.evidence.Root, actual+".stdout")
	expectedContents, err := os.ReadFile(expectedPath)
	if err != nil {
		return err
	}
	actualContents, err := os.ReadFile(actualPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(expectedContents, actualContents) {
		return fmt.Errorf("normalized preservation manifests differ: %s and %s", expected, actual)
	}
	return nil
}

func (state *runnerState) enableGuestRegistry(ctx context.Context, scenario scenarioState) error {
	registry := "10.0.2.2:" + strconv.Itoa(state.options.Ports.Registry)
	script := "install -d -m 0755 /etc/containers/registries.conf.d\n" +
		"printf '%s\\n' '[[registry]]' 'location = \"" + registry + "\"' 'insecure = true' > /etc/containers/registries.conf.d/99-soda-acceptance.conf\n" +
		"chmod 0644 /etc/containers/registries.conf.d/99-soda-acceptance.conf\n"
	return scenario.remote.Sudo(ctx, scenario.password, script, "fallback/registry-enable")
}

func (state *runnerState) disableGuestRegistry(ctx context.Context, scenario scenarioState) error {
	script := "test -f /etc/containers/registries.conf.d/99-soda-acceptance.conf\nrm -- /etc/containers/registries.conf.d/99-soda-acceptance.conf\n"
	return scenario.remote.Sudo(ctx, scenario.password, script, "fallback/registry-disable")
}

func (state *runnerState) switchImage(ctx context.Context, scenario *scenarioState, vm **VM, target string) error {
	reference, digest, err := state.localImageReference(target)
	if err != nil {
		return err
	}
	stageInput := append(bytes.TrimRight(scenario.password, "\r\n"), '\n')
	_, err = scenario.remote.Exchange(ctx, "fallback/"+target+"-download", stageInput,
		"sudo", "-k", "-S", "-p", "", "/usr/bin/bootc", "switch", "--download-only", reference)
	if err != nil {
		return err
	}
	_, err = scenario.remote.Exchange(ctx, "fallback/"+target+"-activate", stageInput,
		"sudo", "-k", "-S", "-p", "", "/usr/bin/bootc", "switch", "--from-downloaded")
	if err != nil {
		return err
	}
	if err = (*vm).PowerDown(ctx); err != nil {
		return err
	}
	boot := "fallback/boot-" + target
	*vm, err = state.launch(ctx, boot, "installed", state.paths.installedDisk, "")
	if err != nil {
		return err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err = scenario.remote.WaitReady(waitCtx); err != nil {
		return err
	}
	return state.assertBootedDigest(ctx, scenario.remote, scenario.password, target, digest)
}

func (state *runnerState) assertBootedDigest(ctx context.Context, remote Remote, password []byte, target, digest string) error {
	input := append(bytes.TrimRight(password, "\r\n"), '\n')
	output, err := remote.Exchange(ctx, "fallback/"+target+"-bootc-status", input,
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

func (state *runnerState) localImageReference(target string) (string, string, error) {
	var digest string
	switch target {
	case "candidate":
		digest = imageDigest(state.artifacts.Candidate)
	case "fallback":
		digest = imageDigest(state.artifacts.Fallback)
	default:
		return "", "", errors.New("fallback target must be candidate or fallback")
	}
	reference := "10.0.2.2:" + strconv.Itoa(state.options.Ports.Registry) + "/soda-os@" + digest
	if strings.ContainsAny(reference, "'\"\\ ") {
		return "", "", errors.New("generated registry reference is unsafe")
	}
	return reference, digest, nil
}
