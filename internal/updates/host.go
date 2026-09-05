package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/process"
)

// Host is a projection of bootc v1 status, not a Soda deployment-state store.
type Host struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Status     HostStatus `json:"status"`
}
type HostStatus struct {
	Booted         *Deployment     `json:"booted"`
	Staged         *Deployment     `json:"staged"`
	RollbackQueued bool            `json:"rollbackQueued"`
	UsrOverlay     json.RawMessage `json:"usrOverlay"`
}
type Deployment struct {
	Image        *ImageStatus `json:"image"`
	DownloadOnly bool         `json:"downloadOnly"`
	Incompatible bool         `json:"incompatible"`
}
type ImageStatus struct {
	Version      string         `json:"version"`
	ImageDigest  string         `json:"imageDigest"`
	Architecture string         `json:"architecture"`
	Image        ImageReference `json:"image"`
}
type ImageReference struct {
	Image     string `json:"image"`
	Transport string `json:"transport"`
}

func ReadHost(ctx context.Context, runner process.Runner) (Host, error) {
	output, err := runner.Output(ctx, process.Command{Name: "/usr/bin/bootc", Args: []string{"status", "--json"}})
	if err != nil {
		return Host{}, err
	}
	var host Host
	if err = json.Unmarshal([]byte(output), &host); err != nil {
		return host, fmt.Errorf("decode bootc status: %w", err)
	}
	if host.APIVersion != "org.containers.bootc/v1" || host.Kind != "BootcHost" || host.Status.Booted == nil || host.Status.Booted.Image == nil {
		return host, errors.New("bootc did not report a booted image; this page requires an installed bootc system")
	}
	return host, nil
}

func (host Host) mutable(architecture string) error {
	if host.Status.RollbackQueued {
		return errors.New("a rollback is queued; resolve it with native bootc before updating")
	}
	if overlay := string(host.Status.UsrOverlay); overlay != "" && overlay != "null" {
		return errors.New("a transient /usr overlay is active; resolve it before updating")
	}
	booted := host.Status.Booted
	// ReadHost has already required a booted image.
	if booted.Incompatible {
		return errors.New("bootc cannot manage the current deployment")
	}
	platform, err := platformFor(architecture)
	if err != nil {
		return err
	}
	if booted.Image.Architecture != strings.TrimPrefix(platform, "linux/") {
		return errors.New("booted image architecture differs from this host")
	}
	if booted.Image.Image.Transport != "registry" || !exactImage.MatchString(booted.Image.Image.Image) {
		return errors.New("Soda Updates requires an exact Soda registry image; use native bootc for other installations")
	}
	return nil
}

func (host Host) requireTarget(selected Release) error {
	staged := host.Status.Staged
	if staged == nil || staged.Image == nil || staged.Incompatible {
		return errors.New("no compatible downloaded deployment; refresh before applying")
	}
	image := staged.Image
	platform, err := platformFor(selected.Architecture)
	if err != nil {
		return err
	}
	if image.Image.Transport != "registry" || image.Image.Image != selected.Reference || repository+"@"+image.ImageDigest != selected.Reference {
		return errors.New("staged image changed; refresh and review the deployment before applying")
	}
	if image.Version != selected.Version || image.Architecture != strings.TrimPrefix(platform, "linux/") {
		return errors.New("staged image identity differs from the verified release")
	}
	return nil
}
