package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/LevitateOS/soda-os/internal/domain"
	"golang.org/x/crypto/ssh"
)

func (s *System) CreatePerson(ctx context.Context, person domain.Person, password string) (domain.GitIdentity, Cleanup, error) {
	if strings.ContainsAny(password, "\r\n\x00") {
		return domain.GitIdentity{}, nil, errors.New("password contains a line or NUL delimiter")
	}
	if utf8.RuneCountInString(password) < 6 {
		return domain.GitIdentity{}, nil, errors.New("password must contain at least 6 characters")
	}
	if _, err := s.Runner.Run(ctx, "useradd", []string{"--create-home", "--shell", "/bin/bash", person.Username}, nil, ""); err != nil {
		return domain.GitIdentity{}, nil, err
	}
	accountCleanup := func(cleanupContext context.Context) error {
		_, err := s.Runner.Run(cleanupContext, "userdel", []string{"--remove", person.Username}, nil, "")
		return err
	}
	if _, err := s.Runner.Run(ctx, "chpasswd", nil, nil, person.Username+":"+password+"\n"); err != nil {
		return domain.GitIdentity{}, nil, failWithCleanup(ctx, err, accountCleanup)
	}
	if _, err := s.Runner.Run(ctx, "chage", []string{"--lastday", "0", person.Username}, nil, ""); err != nil {
		return domain.GitIdentity{}, nil, failWithCleanup(ctx, err, accountCleanup)
	}
	identity, identityCleanup, err := s.createGitIdentity(ctx, person)
	if err != nil {
		return domain.GitIdentity{}, nil, failWithCleanup(ctx, err, accountCleanup)
	}
	return identity, combineCleanups([]Cleanup{accountCleanup, identityCleanup}), nil
}

func (s *System) ImportPerson(ctx context.Context, person domain.Person) (domain.GitIdentity, Cleanup, error) {
	if _, err := s.Runner.Run(ctx, "getent", []string{"passwd", person.Username}, nil, ""); err != nil {
		return domain.GitIdentity{}, nil, fmt.Errorf("%w: Linux account %s", ErrNotFound, person.Username)
	}
	return s.createGitIdentity(ctx, person)
}

func (s *System) createGitIdentity(ctx context.Context, person domain.Person) (domain.GitIdentity, Cleanup, error) {
	if _, err := s.Runner.Run(ctx, "usermod", []string{"--append", "--groups", "soda-people", person.Username}, nil, ""); err != nil {
		return domain.GitIdentity{}, nil, err
	}
	keyPath := s.gitPrivateKeyPath(person.Username)
	cleanup := s.gitIdentityCleanup(person, keyPath)
	identity, err := s.generateGitIdentity(ctx, person, keyPath)
	if err != nil {
		return domain.GitIdentity{}, nil, failWithCleanup(ctx, err, cleanup)
	}
	return identity, cleanup, nil
}

func (s *System) gitIdentityCleanup(person domain.Person, keyPath string) Cleanup {
	return func(cleanupContext context.Context) error {
		_, groupErr := s.Runner.Run(cleanupContext, "gpasswd", []string{"--delete", person.Username, "soda-people"}, nil, "")
		var cleanupErrors []error
		for _, path := range []string{keyPath, keyPath + ".pub"} {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				cleanupErrors = append(cleanupErrors, errors.New("remove personal Git key"))
			}
		}
		cleanupErrors = append(cleanupErrors, groupErr)
		return errors.Join(cleanupErrors...)
	}
}

func (s *System) generateGitIdentity(ctx context.Context, person domain.Person, keyPath string) (domain.GitIdentity, error) {
	sshDir := filepath.Dir(keyPath)
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return domain.GitIdentity{}, errors.New("create personal SSH directory")
	}
	if err := os.Chmod(sshDir, 0o700); err != nil {
		return domain.GitIdentity{}, errors.New("protect personal SSH directory")
	}
	if _, err := s.Runner.Run(ctx, "ssh-keygen", []string{"-q", "-t", "ed25519", "-N", "", "-C", "soda-git-" + person.Username, "-f", keyPath}, nil, ""); err != nil {
		return domain.GitIdentity{}, errors.New("generate personal Git key")
	}
	contents, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return domain.GitIdentity{}, errors.New("read generated Git public key")
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(contents)
	if err != nil {
		return domain.GitIdentity{}, fmt.Errorf("parse generated Git public key: %w", err)
	}
	if _, err = s.Runner.Run(ctx, "chown", []string{"--recursive", person.Username, sshDir}, nil, ""); err != nil {
		return domain.GitIdentity{}, errors.New("assign personal Git key ownership")
	}
	if _, err = s.Runner.Run(ctx, "restorecon", []string{"-R", sshDir}, nil, ""); err != nil {
		return domain.GitIdentity{}, errors.New("label personal Git key")
	}
	return domain.GitIdentity{PersonID: person.ID, PublicKey: strings.TrimSpace(string(contents)), Fingerprint: ssh.FingerprintSHA256(publicKey)}, nil
}
