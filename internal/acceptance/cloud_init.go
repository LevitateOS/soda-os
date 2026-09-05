package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The local-forwarded fixture uses VM tooling to deliver ordinary cloud-init input.
func prepareQCOW2UserData(ctx context.Context, admin personFixture, work string) (string, error) {
	hash, err := CommandOutput(ctx, CommandSpec{Name: "openssl", Args: []string{"passwd", "-6", "-stdin"}, Stdin: bytes.NewReader(admin.LinuxPassword)})
	if err != nil {
		return "", fmt.Errorf("hash cloud-init fixture password: %w", err)
	}
	config := map[string]any{
		"users": []any{map[string]any{
			"name": admin.Remote.Username, "groups": []string{"wheel"}, "shell": "/bin/bash",
			"ssh_authorized_keys": []string{strings.TrimSpace(string(admin.PublicKey))},
			"lock_passwd":         false, "hashed_passwd": strings.TrimSpace(string(hash)),
		}},
		"disable_root": true,
	}
	body, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	userData := filepath.Join(work, "qcow-user-data")
	if err = os.WriteFile(userData, append([]byte("#cloud-config\n"), body...), 0600); err != nil {
		return "", err
	}
	seed := filepath.Join(work, "qcow-cloud-init.iso")
	if err = RunCommand(ctx, CommandSpec{Name: "cloud-localds", Args: []string{seed, userData}}); err != nil {
		return "", err
	}
	if err = os.Chmod(seed, 0600); err != nil {
		return "", err
	}
	return seed, nil
}
