package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LevitateOS/soda-os/internal/domain"
)

func (s *System) createProjectKeys(ctx context.Context, project domain.Project, root string) error {
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
	return s.createAuthorizedKeysFile(project)
}

func (s *System) createAuthorizedKeysFile(project domain.Project) error {
	if err := os.MkdirAll(s.AuthorizedKeysRoot, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(s.AuthorizedKeysRoot, 0o755); err != nil {
		return err
	}
	keyFile := s.authorizedKeysPath(project)
	file, err := os.OpenFile(keyFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Chmod(keyFile, 0o644)
}

func (s *System) ReconcileAuthorizedKeys(ctx context.Context, project domain.Project, access []domain.ProjectAccess) error {
	s.authorizedKeysMu.Lock()
	defer s.authorizedKeysMu.Unlock()
	return s.writeAuthorizedKeys(ctx, s.authorizedKeysPath(project), s.authorizedKeyContents(project, access))
}

func (s *System) authorizedKeyContents(project domain.Project, access []domain.ProjectAccess) string {
	sort.Slice(access, func(i, j int) bool { return access[i].Person.Username < access[j].Person.Username })
	lines := make([]string, 0)
	for _, entry := range access {
		lines = append(lines, s.authorizedKeyLines(project, entry)...)
	}
	if len(lines) != 0 {
		return strings.Join(lines, "\n") + "\n"
	}
	return ""
}

func (s *System) authorizedKeyLines(project domain.Project, access domain.ProjectAccess) []string {
	keys := append([]domain.SSHDeviceKey(nil), access.Keys...)
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Label == keys[j].Label {
			return keys[i].Fingerprint < keys[j].Fingerprint
		}
		return keys[i].Label < keys[j].Label
	})
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, s.authorizedKeyLine(project, access.Person, access.Worktree, key))
	}
	return lines
}

func (s *System) authorizedKeyLine(project domain.Project, person domain.Person, tree domain.Worktree, key domain.SSHDeviceKey) string {
	home := s.sessionHome(project, person)
	command := fmt.Sprintf("/usr/libexec/soda/soda-ssh --actor %s --project %s --worktree %s --home %s", person.Username, project.Slug, tree.Path, home)
	return fmt.Sprintf("command=\"%s\" %s", command, key.PublicKey)
}

func (s *System) writeAuthorizedKeys(ctx context.Context, path, contents string) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".authorized-keys-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	// OpenSSH reads this root-owned file after switching to the project account.
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
