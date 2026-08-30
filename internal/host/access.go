package host

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LevitateOS/soda-os/internal/domain"
)

func (s *System) createProjectKeys(ctx context.Context, project domain.Project, root string) error {
	if _, builtIn := project.Source.(domain.EmptyProjectSource); !builtIn {
		return nil
	}
	sshDir := filepath.Join(root, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(sshDir, 0o700); err != nil {
		return err
	}
	if _, err := s.Runner.Run(ctx, "ssh-keygen", []string{"-q", "-t", "ed25519", "-N", "", "-C", "soda-project-" + project.Slug, "-f", filepath.Join(sshDir, "deploy_key")}, nil, ""); err != nil {
		return err
	}
	return nil
}

func (s *System) ReconcileAuthorizedKeys(ctx context.Context, person domain.Person, keys []domain.SSHDeviceKey) error {
	s.authorizedKeysMu.Lock()
	defer s.authorizedKeysMu.Unlock()
	if err := os.MkdirAll(s.AuthorizedKeysRoot, 0o755); err != nil {
		return err
	}
	keys = append([]domain.SSHDeviceKey(nil), keys...)
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Label == keys[j].Label {
			return keys[i].Fingerprint < keys[j].Fingerprint
		}
		return keys[i].Label < keys[j].Label
	})
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key.PublicKey)
	}
	contents := ""
	if len(lines) != 0 {
		contents = strings.Join(lines, "\n") + "\n"
	}
	return s.writeAuthorizedKeys(ctx, s.authorizedKeysPath(person.Username), contents)
}

func (s *System) writeAuthorizedKeys(ctx context.Context, path, contents string) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".authorized-keys-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	// OpenSSH reads this root-owned file while authenticating the person account.
	// Public keys are not secret; world-readability preserves root-only writes.
	if err = temporary.Chmod(0o644); err == nil {
		_, err = temporary.WriteString(contents)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return err
	}
	if _, err = s.Runner.Run(ctx, "chown", []string{"root:root", path}, nil, ""); err == nil {
		_, err = s.Runner.Run(ctx, "restorecon", []string{path}, nil, "")
	}
	return err
}
