package acceptance

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Secret struct {
	Label string
	Value []byte
}

type Evidence struct {
	Root string
}

func CreateEvidence(path string) (Evidence, error) {
	if path == "" {
		return Evidence{}, errors.New("evidence directory is required")
	}
	if _, err := os.Lstat(path); err == nil {
		return Evidence{}, fmt.Errorf("evidence directory already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Evidence{}, fmt.Errorf("inspect evidence directory: %w", err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return Evidence{}, fmt.Errorf("create evidence directory: %w", err)
	}
	root, err := filepath.Abs(path)
	return Evidence{Root: root}, err
}

func (evidence Evidence) Write(relative string, contents []byte) error {
	path, err := evidence.path(relative)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create evidence parent: %w", err)
	}
	if err = os.WriteFile(path, contents, 0o600); err != nil {
		return fmt.Errorf("write evidence %s: %w", relative, err)
	}
	return nil
}

func (evidence Evidence) path(relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", errors.New("evidence path must be relative")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("evidence path escapes its directory")
	}
	return filepath.Join(evidence.Root, clean), nil
}

func (evidence Evidence) Sanitize(secrets []Secret) error {
	redacted := []string{}
	err := filepath.WalkDir(evidence.Root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !entry.Type().IsRegular() || entry.Name() == "secret-absence.txt" {
			return walkErr
		}
		matched, err := sanitizeFile(path, secrets)
		if err != nil {
			return err
		}
		if len(matched) > 0 {
			relative, _ := filepath.Rel(evidence.Root, path)
			redacted = append(redacted, relative+":"+strings.Join(matched, ","))
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(redacted)
	return evidence.writeSanitizationReport(secrets, redacted)
}

func sanitizeFile(path string, secrets []Secret) ([]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	original := append([]byte(nil), contents...)
	matched := []string{}
	for _, secret := range secrets {
		value := bytes.TrimRight(secret.Value, "\r\n")
		if len(value) > 0 && bytes.Contains(contents, value) {
			contents = bytes.ReplaceAll(contents, value, []byte("[REDACTED]"))
			matched = append(matched, secret.Label)
		}
	}
	if !bytes.Equal(original, contents) {
		err = os.WriteFile(path, contents, 0o600)
	}
	sort.Strings(matched)
	return matched, err
}

func (evidence Evidence) writeSanitizationReport(secrets []Secret, redacted []string) error {
	var report strings.Builder
	if len(redacted) == 0 {
		report.WriteString("result=pass\n")
		for _, secret := range secrets {
			fmt.Fprintf(&report, "%s=absent\n", secret.Label)
		}
	} else {
		report.WriteString("result=fail-redacted\n")
		for _, item := range redacted {
			fmt.Fprintf(&report, "redacted=%s\n", item)
		}
	}
	if err := evidence.Write("secret-absence.txt", []byte(report.String())); err != nil {
		return err
	}
	if len(redacted) > 0 {
		return errors.New("credential material reached evidence and was redacted")
	}
	return nil
}
