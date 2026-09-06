package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LevitateOS/soda-os/internal/process"
)

// Host is a projection of bootc v1 status, not a Soda deployment-state store.
type Host struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Spec       HostSpec   `json:"spec"`
	Status     HostStatus `json:"status"`
}
type HostSpec struct {
	Image *ImageReference `json:"image"`
}
type HostStatus struct {
	Booted         *Deployment     `json:"booted"`
	Staged         *Deployment     `json:"staged"`
	RollbackQueued bool            `json:"rollbackQueued"`
	UsrOverlay     json.RawMessage `json:"usrOverlay"`
	ReadOnly       bool            `json:"readOnly"`
}
type Deployment struct {
	Image        *ImageStatus `json:"image"`
	CachedUpdate *ImageStatus `json:"cachedUpdate"`
	DownloadOnly bool         `json:"downloadOnly"`
	Incompatible bool         `json:"incompatible"`
}
type ImageStatus struct {
	Version      *string        `json:"version"`
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
	if host.APIVersion != "org.containers.bootc/v1" || host.Kind != "BootcHost" || host.Status.Booted == nil {
		return host, errors.New("bootc did not report a booted deployment; this page requires an installed bootc system")
	}
	return host, nil
}

func (host Host) mutable() error {
	if host.Status.RollbackQueued {
		return errors.New("a rollback is queued; resolve it with native bootc before updating")
	}
	if overlay := string(host.Status.UsrOverlay); overlay != "" && overlay != "null" {
		return errors.New("a /usr overlay is active; resolve it before updating")
	}
	if host.Status.ReadOnly {
		return errors.New("the bootc system is read-only; updating is unavailable")
	}
	if host.Status.Booted.Incompatible || host.Status.Booted.Image == nil {
		return errors.New("bootc cannot manage the current deployment")
	}
	if staged := host.Status.Staged; staged != nil && staged.Incompatible {
		return errors.New("bootc cannot manage the staged deployment; inspect native status before updating")
	}
	if host.Spec.Image == nil {
		return errors.New("no native image source is configured; inspect bootc status")
	}
	return nil
}
