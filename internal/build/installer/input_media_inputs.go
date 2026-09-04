package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LevitateOS/soda-os/internal/process"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type installerInputReleaseRecord struct {
	SchemaVersion uint32 `json:"schema_version"`
	Platform      string `json:"platform"`
	ISOChecksum   string `json:"iso_sha256"`
}

func validateInstallerInputOutput(path string) error {
	if path == "" {
		return errors.New("installer input output path is required")
	}
	if err := requireInstallerInputOutputAbsent(path); err != nil {
		return err
	}
	return requireInstallerInputOutputDirectory(filepath.Dir(path))
}

func requireInstallerInputOutputAbsent(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return errors.New("installer input output already exists")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect installer input output: %w", err)
	}
	return nil
}

func requireInstallerInputOutputDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect installer input output directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("installer input output parent is not a directory")
	}
	return nil
}

func validateInstallerInputArtifact(isoPath, recordPath, expectedPlatform string) error {
	if !regularFile(isoPath) {
		return errors.New("installer ISO is not a regular file")
	}
	if !regularFile(recordPath) {
		return errors.New("installer release record is not a regular file")
	}
	record, err := readInstallerInputReleaseRecord(recordPath)
	if err != nil {
		return err
	}
	if err = validateInstallerInputReleaseRecord(record, expectedPlatform); err != nil {
		return err
	}
	return validateInstallerInputChecksum(isoPath, record.ISOChecksum)
}

func validateInstallerInputChecksum(isoPath, expected string) error {
	actual, err := fileSHA256(isoPath)
	if err != nil {
		return fmt.Errorf("checksum installer ISO: %w", err)
	}
	if actual != expected {
		return errors.New("installer ISO checksum differs from its release record")
	}
	return nil
}

func readInstallerInputReleaseRecord(path string) (installerInputReleaseRecord, error) {
	recordFile, err := os.Open(path)
	if err != nil {
		return installerInputReleaseRecord{}, fmt.Errorf("open installer release record: %w", err)
	}
	defer recordFile.Close()

	var record installerInputReleaseRecord
	decoder := json.NewDecoder(recordFile)
	if err = decoder.Decode(&record); err != nil {
		return installerInputReleaseRecord{}, fmt.Errorf("decode installer release record: %w", err)
	}
	if err = requireJSONEOF(decoder); err != nil {
		return installerInputReleaseRecord{}, err
	}
	return record, nil
}

func validateInstallerInputReleaseRecord(record installerInputReleaseRecord, expectedPlatform string) error {
	if record.SchemaVersion != 4 {
		return errors.New("installer release record schema is not supported")
	}
	if record.Platform != expectedPlatform {
		return errors.New("installer release record platform differs from the selected architecture")
	}
	if !validInstallerInputChecksum(record.ISOChecksum) {
		return errors.New("installer release record ISO checksum is invalid")
	}
	return nil
}

func validInstallerInputChecksum(checksum string) bool {
	if len(checksum) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(checksum)
	return err == nil && checksum == strings.ToLower(checksum)
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode installer release record: %w", err)
	}
	return errors.New("installer release record contains more than one JSON value")
}

func (b *InstallerInputBuilder) readCanonicalPublicKey(ctx context.Context, path string) ([]byte, error) {
	if err := validateInstallerInputPublicKeyFile(path); err != nil {
		return nil, err
	}
	if err := b.validatePublicKeyWithOpenSSH(ctx, path); err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read administrator SSH public key: %w", err)
	}
	return canonicalInstallerInputPublicKey(contents)
}

func validateInstallerInputPublicKeyFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return errors.New("administrator SSH public key is not a regular file")
	}
	if !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 16*1024 {
		return errors.New("administrator SSH public key is not a regular file")
	}
	return nil
}

func (b *InstallerInputBuilder) validatePublicKeyWithOpenSSH(ctx context.Context, path string) error {
	_, err := b.runner.Output(ctx, process.Command{
		Name: "ssh-keygen",
		Args: []string{"-l", "-f", path},
	})
	if err != nil {
		return errors.New("administrator SSH public key is invalid")
	}
	return nil
}

func canonicalInstallerInputPublicKey(contents []byte) ([]byte, error) {
	contents, err := trimOneTerminalNewline(contents, "administrator SSH public key")
	if err != nil {
		return nil, err
	}
	key, _, options, rest, err := ssh.ParseAuthorizedKey(contents)
	if err != nil || len(options) != 0 || len(rest) != 0 {
		return nil, errors.New("administrator SSH public key is invalid")
	}
	return []byte(key.Type() + " " + base64.StdEncoding.EncodeToString(key.Marshal())), nil
}

func readProtectedSecret(path, label string) ([]byte, error) {
	info, err := inspectProtectedSecret(path, label)
	if err != nil {
		return nil, err
	}
	if err = validateProtectedSecretFile(info, label); err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s file: %w", label, err)
	}
	return trimOneTerminalNewline(contents, label)
}

func inspectProtectedSecret(path, label string) (os.FileInfo, error) {
	if path == "" {
		return nil, fmt.Errorf("%s file is required", label)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect %s file: %w", label, err)
	}
	return info, nil
}

func validateProtectedSecretFile(info os.FileInfo, label string) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s input must be a regular file, not a symlink", label)
	}
	if info.Size() == 0 || info.Size() > 4096 {
		return fmt.Errorf("%s file has an invalid size", label)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s file must not be accessible by group or other users", label)
	}
	return nil
}

func trimOneTerminalNewline(contents []byte, label string) ([]byte, error) {
	contents = trimTerminalNewline(contents)
	if len(contents) == 0 {
		return nil, fmt.Errorf("%s must not be empty", label)
	}
	if bytes.IndexAny(contents, "\x00\r\n") >= 0 {
		return nil, fmt.Errorf("%s must contain exactly one value", label)
	}
	return contents, nil
}

func trimTerminalNewline(contents []byte) []byte {
	if len(contents) > 0 && contents[len(contents)-1] == '\n' {
		contents = contents[:len(contents)-1]
	}
	if len(contents) > 0 && contents[len(contents)-1] == '\r' {
		contents = contents[:len(contents)-1]
	}
	return contents
}

func (b *InstallerInputBuilder) administratorPassword(path string) ([]byte, error) {
	if path != "" {
		return readProtectedSecret(path, "administrator password")
	}
	password, err := b.readPasswordValue("Administrator password: ", "administrator password")
	if err != nil {
		return nil, err
	}
	confirmation, err := b.readPasswordValue("Confirm administrator password: ", "administrator password confirmation")
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(password, confirmation) {
		return nil, errors.New("administrator password confirmation does not match")
	}
	return password, nil
}

func (b *InstallerInputBuilder) readPasswordValue(prompt, label string) ([]byte, error) {
	password, err := b.passwordReader(prompt)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	return trimOneTerminalNewline(password, label)
}

func readPasswordFromTerminal(prompt string) ([]byte, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, errors.New("controlling terminal is unavailable")
	}
	defer tty.Close()
	if _, err = fmt.Fprint(tty, prompt); err != nil {
		return nil, errors.New("write password prompt")
	}
	password, err := term.ReadPassword(int(tty.Fd()))
	_, newlineErr := fmt.Fprintln(tty)
	if err != nil {
		return nil, errors.New("read password from controlling terminal")
	}
	if newlineErr != nil {
		return nil, errors.New("finish password prompt")
	}
	return password, nil
}
