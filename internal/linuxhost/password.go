package linuxhost

import (
	"context"
	"fmt"
	"strings"
)

type PasswordStatus uint8

const (
	PasswordLocked PasswordStatus = iota + 1
	PasswordSet
	PasswordUnset
)

func (native *Native) PasswordStatus(ctx context.Context, account Account) (PasswordStatus, error) {
	result, err := native.run(ctx, "/usr/bin/passwd", "--status", account.Username)
	if err != nil {
		return 0, fmt.Errorf("read Linux password status for %s: %w", account.Username, err)
	}
	if result.ExitCode != 0 {
		return 0, fmt.Errorf("read Linux password status for %s: %s", account.Username, strings.TrimSpace(result.Stderr))
	}
	fields := strings.Fields(result.Stdout)
	if len(fields) < 2 || fields[0] != account.Username {
		return 0, fmt.Errorf("Linux password status for %s is invalid", account.Username)
	}
	switch fields[1] {
	case "L", "LK":
		return PasswordLocked, nil
	case "P", "PS":
		return PasswordSet, nil
	case "NP":
		return PasswordUnset, nil
	default:
		return 0, fmt.Errorf("Linux password status for %s is unknown", account.Username)
	}
}
