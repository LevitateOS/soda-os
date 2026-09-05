package acceptance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// guest owns one disk's VM lifetime, including replacement processes during fallback.
// The itinerary and finalizer use it sequentially; only cleanup itself is once-only.
// ISO and reusable QCOW2 instances never share an enrollment obligation.
type guest struct {
	vm          *VM
	config      VMConfig
	evidence    Evidence
	enrollment  *guestEnrollment
	cleanupOnce sync.Once
	cleanupErr  error
}

type guestEnrollment struct {
	remote    Remote
	password  []byte
	attempted bool
	err       error
}

func launchGuest(ctx context.Context, config VMConfig, evidence Evidence, cleanup *Cleanup) (*guest, error) {
	vm, err := LaunchVM(ctx, config)
	if err != nil {
		return nil, err
	}
	guest := &guest{vm: vm, config: config, evidence: evidence}
	if err = cleanup.Add(CleanupAction{Name: "guest " + config.Mode, Run: guest.cleanup}); err != nil {
		return nil, errors.Join(err, guest.cleanup(ctx))
	}
	return guest, nil
}

// restart preserves the enrollment and disk. It never logs out of Tailscale.
// A failed powerdown retains the old VM for emergency stop; a failed launch has
// no live replacement (LaunchVM owns cleanup of its partial launch).
func (guest *guest) restart(ctx context.Context, relative string) error {
	if err := guest.powerDown(ctx); err != nil {
		return err
	}
	config := guest.config
	config.Mode, config.ISO = "installed", ""
	directory, err := guest.evidence.path(relative)
	if err != nil {
		return err
	}
	config.Directory = directory
	if err = os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	vm, err := LaunchVM(ctx, config)
	if err != nil {
		return err
	}
	guest.vm = vm
	return nil
}

// shutdown is the normal itinerary endpoint, unlike a fallback restart.
// Failure to log out must not prevent attempting to power down the exact VM.
func (guest *guest) shutdown(ctx context.Context) error {
	return errors.Join(guest.logout(ctx), guest.powerDown(ctx))
}

func (guest *guest) powerDown(ctx context.Context) error {
	if guest.vm == nil {
		return nil
	}
	if err := guest.vm.PowerDown(ctx); err != nil {
		return err
	}
	guest.vm = nil
	return nil
}

func (guest *guest) logout(ctx context.Context) error {
	enrollment := guest.enrollment
	if enrollment == nil {
		return nil
	}
	if !enrollment.attempted {
		enrollment.attempted = true
		logoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		enrollment.err = enrollment.remote.Sudo(logoutCtx, enrollment.password, "/usr/bin/tailscale logout\n", "cleanup/tailscale-logout")
	}
	if enrollment.err != nil {
		return fmt.Errorf("revoke guest Tailnet enrollment: %w", enrollment.err)
	}
	return nil
}

// Cleanup gets independent bounded attempts for logout and process stop. An
// expired logout deadline or cancelled itinerary cannot skip process cleanup.
// Keep failures even on repeated calls; an attempted logout is not a success.
func (guest *guest) cleanup(ctx context.Context) error {
	guest.cleanupOnce.Do(func() {
		logoutErr := guest.logout(context.WithoutCancel(ctx))
		var stopErr error
		if guest.vm != nil {
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			stopErr = guest.vm.Stop(stopCtx)
			cancel()
			if stopErr == nil {
				guest.vm = nil
			}
		}
		guest.cleanupErr = errors.Join(logoutErr, stopErr)
	})
	return guest.cleanupErr
}

func (guest *guest) captureQMP(ctx context.Context, relative string) error {
	var status map[string]any
	if err := guest.vm.QMP.Execute(ctx, "query-status", "status", nil, &status); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return guest.evidence.Write(relative, append(contents, '\n'))
}
