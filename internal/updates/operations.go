package updates

import (
	"context"
	"errors"
	"fmt"

	"github.com/LevitateOS/soda-os/internal/process"
)

// Selection contains only the user-confirmed release identity. It grants no
// authority: publication and host state are verified again for each operation.
type Selection struct {
	Version   string `json:"version"`
	Reference string `json:"reference"`
}

type PublishedReleases interface {
	Published(context.Context, string, string) (Release, error)
}

type Operations struct {
	Runner       process.Runner
	Releases     PublishedReleases
	Architecture string
}

func (o Operations) selected(ctx context.Context, selection Selection) (Release, error) {
	if !stableVersion.MatchString(selection.Version) || !exactImage.MatchString(selection.Reference) {
		return Release{}, errors.New("select an exact published Soda release")
	}
	selected, err := o.Releases.Published(ctx, o.Architecture, selection.Version)
	if err != nil {
		return Release{}, err
	}
	if selected.Reference != selection.Reference {
		return Release{}, errors.New("release digest changed since selection; check again")
	}
	return selected, nil
}

func (o Operations) Download(ctx context.Context, selection Selection) error {
	selected, err := o.selected(ctx, selection)
	if err != nil {
		return err
	}
	host, err := o.host(ctx)
	if err != nil {
		return err
	}
	if host.Status.Staged != nil {
		return errors.New("a deployment is already staged; refresh and review it before another download")
	}
	comparison, err := CompareStableVersions(host.Status.Booted.Image.Version, selected.Version)
	if err != nil {
		return err
	}
	if comparison >= 0 {
		return errors.New("selected release is not newer than the booted version")
	}
	if err = o.Runner.Run(ctx, process.Command{Name: "/usr/bin/bootc", Args: []string{"switch", "--download-only", selected.Reference}}); err != nil {
		return err
	}
	host, err = o.host(ctx)
	if err != nil {
		return err
	}
	if err = host.requireTarget(selected); err != nil {
		return err
	}
	if !host.Status.Staged.DownloadOnly {
		return errors.New("deployment is already enabled for next restart; review native bootc status")
	}
	return nil
}

// Apply rechecks the native target before and after unlocking, then requests a
// normal restart. bootc 1.16.10 has no expected-digest activation argument:
// administrators must not concurrently run other deployment tools during Apply.
// The caller serializes Soda operations; this is not a lock on external bootc.
func (o Operations) Apply(ctx context.Context, selection Selection) error {
	selected, err := o.selected(ctx, selection)
	if err != nil {
		return err
	}
	host, err := o.host(ctx)
	if err != nil {
		return err
	}
	if err = host.requireTarget(selected); err != nil {
		return err
	}
	if err = o.Runner.Run(ctx, process.Command{Name: "/usr/bin/bootc", Args: []string{"switch", "--from-downloaded"}}); err != nil {
		return err
	}
	if err = o.requireAppliedTarget(ctx, selected); err != nil {
		return fmt.Errorf("activation may have changed pending deployment state; restart was NOT requested; inspect bootc before any restart: %w", err)
	}
	if err = o.Runner.Run(ctx, process.Command{Name: "/usr/bin/systemctl", Args: []string{"reboot"}}); err != nil {
		return fmt.Errorf("update is enabled for next restart, but restart request failed: %w", err)
	}
	return nil
}

func (o Operations) host(ctx context.Context) (Host, error) {
	host, err := ReadHost(ctx, o.Runner)
	if err != nil {
		return host, err
	}
	return host, host.mutable(o.Architecture)
}
func (o Operations) requireAppliedTarget(ctx context.Context, selected Release) error {
	host, err := o.host(ctx)
	if err != nil {
		return err
	}
	if err = host.requireTarget(selected); err != nil {
		return err
	}
	if host.Status.Staged.DownloadOnly {
		return errors.New("deployment remains download-only")
	}
	return nil
}
