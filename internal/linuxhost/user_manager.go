package linuxhost

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func (native *Native) resetFailedUserManager(ctx context.Context, account Account) error {
	unit := "user@" + strconv.Itoa(account.UID) + ".service"
	state, err := native.userManagerActiveState(ctx, unit)
	if err != nil {
		return err
	}
	if state == "inactive" {
		return nil
	}
	if state != "failed" {
		return fmt.Errorf("inspect %s failure state: unexpected active state %q", unit, state)
	}
	result, err := native.run(ctx, "/usr/bin/systemctl", "reset-failed", unit)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		current, inspectErr := native.userManagerActiveState(ctx, unit)
		if inspectErr == nil && current == "inactive" {
			return nil
		}
		return fmt.Errorf("reset %s failure state: %s", unit, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (native *Native) userManagerActiveState(ctx context.Context, unit string) (string, error) {
	result, err := native.run(ctx, "/usr/bin/systemctl", "show", "--property=ActiveState", "--value", unit)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("inspect %s failure state: %s", unit, strings.TrimSpace(result.Stderr))
	}
	state := strings.TrimSpace(result.Stdout)
	if state == "" || strings.ContainsAny(state, "\r\n") {
		return "", fmt.Errorf("inspect %s failure state: unexpected active state %q", unit, state)
	}
	return state, nil
}
