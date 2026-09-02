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
	"regexp"
	"runtime"
	"strings"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

const installerInputKickstartPrefix = `%include /usr/share/anaconda/interactive-defaults.ks
`

const installerInputUnattendedCommands = `# Fixed destructive storage automation for disposable acceptance VMs only.
cmdline
lang en_US.UTF-8
keyboard us
timezone UTC --utc
network --bootproto=dhcp --device=link --activate --onboot=on --hostname=soda-acceptance
zerombr
clearpart --all --initlabel
autopart --type=plain --fstype=ext4
eula --agreed
reboot
`

const installerInputKickstartScripts = `%pre --erroronfail
/usr/libexec/soda/soda-installer-input
%end
%include /run/soda-installer/account.ks
%post --nochroot --erroronfail
/usr/libexec/soda/soda-installer-finalize
%end
`

var installerInputUsernamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

type InstallerInputOptions struct {
	ISOPath              string
	ReleaseRecordPath    string
	Username             string
	SSHPublicKeyPath     string
	TailscaleAuthKeyPath string
	PasswordPath         string
	OutputPath           string
	Unattended           bool
}

type PasswordReader func(string) ([]byte, error)

type InstallerInputBuilder struct {
	Spec             config.DistroSpec
	hostArchitecture string
	runner           process.Runner
	passwordReader   PasswordReader
}

func NewInstallerInputBuilder(spec config.DistroSpec, runner process.Runner, passwordReader PasswordReader) *InstallerInputBuilder {
	if runner == nil {
		runner = process.OSRunner{}
	}
	if passwordReader == nil {
		passwordReader = readPasswordFromTerminal
	}
	return &InstallerInputBuilder{
		Spec:             spec,
		hostArchitecture: runtime.GOARCH,
		runner:           runner,
		passwordReader:   passwordReader,
	}
}

func (b *InstallerInputBuilder) Build(ctx context.Context, options InstallerInputOptions) (string, error) {
	if err := b.validateArchitecture(); err != nil {
		return "", err
	}
	if err := validateInstallerInputOutput(options.OutputPath); err != nil {
		return "", err
	}
	if err := validateInstallerInputArtifact(options.ISOPath, options.ReleaseRecordPath, b.Spec.Platform.Architecture.Platform); err != nil {
		return "", err
	}
	if !installerInputUsernamePattern.MatchString(options.Username) {
		return "", errors.New("administrator username does not match the Soda account contract")
	}
	publicKey, err := b.readCanonicalPublicKey(ctx, options.SSHPublicKeyPath)
	if err != nil {
		return "", err
	}
	tailscaleAuthKey, err := readProtectedSecret(options.TailscaleAuthKeyPath, "Tailscale auth key")
	if err != nil {
		return "", err
	}
	password, err := b.administratorPassword(options.PasswordPath)
	if err != nil {
		return "", err
	}

	files := map[string][]byte{
		"ks.cfg":                            []byte(installerInputKickstart(options.Unattended)),
		"soda/administrator-username":       []byte(options.Username),
		"soda/administrator-password":       password,
		"soda/administrator-authorized-key": publicKey,
		"soda/tailscale-auth-key":           tailscaleAuthKey,
	}
	if err := b.createMedia(ctx, options.OutputPath, files); err != nil {
		return "", err
	}
	return options.OutputPath, nil
}

func installerInputKickstart(unattended bool) string {
	kickstart := installerInputKickstartPrefix
	if unattended {
		kickstart += installerInputUnattendedCommands
	}
	return kickstart + installerInputKickstartScripts
}

func (b *InstallerInputBuilder) validateArchitecture() error {
	architecture := b.Spec.Platform.Architecture
	if b.Spec.Identity.Architecture != architecture.Name || b.Spec.Base.Platform != architecture.Platform {
		return errors.New("installer input architecture differs from the selected Soda platform")
	}
	hostArchitecture := b.hostArchitecture
	if hostArchitecture == "" {
		hostArchitecture = runtime.GOARCH
	}
	return config.RequireNativeHostArchitecture(architecture.Name, hostArchitecture)
}

func validateInstallerInputOutput(path string) error {
	if path == "" {
		return errors.New("installer input output path is required")
	}
	if _, err := os.Lstat(path); err == nil {
		return errors.New("installer input output already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect installer input output: %w", err)
	}
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("inspect installer input output directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("installer input output parent is not a directory")
	}
	return nil
}

type installerInputReleaseRecord struct {
	SchemaVersion uint32 `json:"schema_version"`
	Platform      string `json:"platform"`
	ISOChecksum   string `json:"iso_sha256"`
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
	actual, err := fileSHA256(isoPath)
	if err != nil {
		return fmt.Errorf("checksum installer ISO: %w", err)
	}
	if actual != record.ISOChecksum {
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
	if record.SchemaVersion != 2 {
		return errors.New("installer release record schema is not supported")
	}
	if record.Platform != expectedPlatform {
		return errors.New("installer release record platform differs from the selected architecture")
	}
	if len(record.ISOChecksum) != sha256.Size*2 {
		return errors.New("installer release record ISO checksum is invalid")
	}
	if _, err := hex.DecodeString(record.ISOChecksum); err != nil || record.ISOChecksum != strings.ToLower(record.ISOChecksum) {
		return errors.New("installer release record ISO checksum is invalid")
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode installer release record: %w", err)
	}
	return errors.New("installer release record contains more than one JSON value")
}

func (b *InstallerInputBuilder) readCanonicalPublicKey(ctx context.Context, path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 16*1024 {
		return nil, errors.New("administrator SSH public key is not a regular file")
	}
	if _, err = b.runner.Output(ctx, process.Command{
		Name: "ssh-keygen",
		Args: []string{"-l", "-f", path},
	}); err != nil {
		return nil, errors.New("administrator SSH public key is invalid")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read administrator SSH public key: %w", err)
	}
	contents, err = trimOneTerminalNewline(contents, "administrator SSH public key")
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
	if path == "" {
		return nil, fmt.Errorf("%s file is required", label)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect %s file: %w", label, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s input must be a regular file, not a symlink", label)
	}
	if info.Size() == 0 || info.Size() > 4096 {
		return nil, fmt.Errorf("%s file has an invalid size", label)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s file must not be accessible by group or other users", label)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s file: %w", label, err)
	}
	return trimOneTerminalNewline(contents, label)
}

func trimOneTerminalNewline(contents []byte, label string) ([]byte, error) {
	if len(contents) > 0 && contents[len(contents)-1] == '\n' {
		contents = contents[:len(contents)-1]
	}
	if len(contents) > 0 && contents[len(contents)-1] == '\r' {
		contents = contents[:len(contents)-1]
	}
	if len(contents) == 0 {
		return nil, fmt.Errorf("%s must not be empty", label)
	}
	if bytes.IndexAny(contents, "\x00\r\n") >= 0 {
		return nil, fmt.Errorf("%s must contain exactly one value", label)
	}
	return contents, nil
}

func (b *InstallerInputBuilder) administratorPassword(path string) ([]byte, error) {
	if path != "" {
		return readProtectedSecret(path, "administrator password")
	}
	password, err := b.passwordReader("Administrator password: ")
	if err != nil {
		return nil, fmt.Errorf("read administrator password: %w", err)
	}
	password, err = trimOneTerminalNewline(password, "administrator password")
	if err != nil {
		return nil, err
	}
	confirmation, err := b.passwordReader("Confirm administrator password: ")
	if err != nil {
		return nil, fmt.Errorf("read administrator password confirmation: %w", err)
	}
	confirmation, err = trimOneTerminalNewline(confirmation, "administrator password confirmation")
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(password, confirmation) {
		return nil, errors.New("administrator password confirmation does not match")
	}
	return password, nil
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

func (b *InstallerInputBuilder) createMedia(ctx context.Context, outputPath string, files map[string][]byte) (resultErr error) {
	outputAbsolute, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve installer input output: %w", err)
	}
	work, staging, err := stageInstallerInputFiles(outputAbsolute, files)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if cleanupErr := os.RemoveAll(work); cleanupErr != nil {
			if published {
				_ = os.Remove(outputAbsolute)
			}
			cleanupErr = fmt.Errorf("remove private installer input workspace: %w", cleanupErr)
			if resultErr == nil {
				resultErr = cleanupErr
			} else {
				resultErr = fmt.Errorf("%v; %w", resultErr, cleanupErr)
			}
		}
	}()
	temporaryISO := filepath.Join(work, "installer-input.iso")
	if err = b.runInstallerInputXorriso(ctx, staging, temporaryISO); err != nil {
		return err
	}
	if err = b.verifyMedia(ctx, work, temporaryISO, files); err != nil {
		return err
	}
	if err = removeInstallerInputPlaintext(work, staging); err != nil {
		return err
	}
	if err = publishInstallerInputISO(temporaryISO, outputAbsolute); err != nil {
		return err
	}
	published = true
	return nil
}

func removeInstallerInputPlaintext(work, staging string) error {
	for _, path := range []string{staging, filepath.Join(work, "verified")} {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove staged installer input plaintext: %w", err)
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			if err == nil {
				return errors.New("staged installer input plaintext remains")
			}
			return fmt.Errorf("verify staged installer input plaintext removal: %w", err)
		}
	}
	return nil
}

func stageInstallerInputFiles(outputAbsolute string, files map[string][]byte) (string, string, error) {
	work, err := os.MkdirTemp(filepath.Dir(outputAbsolute), ".soda-installer-input-*")
	if err != nil {
		return "", "", fmt.Errorf("create private installer input workspace: %w", err)
	}
	staging := filepath.Join(work, "input")
	if err = os.Mkdir(staging, 0o700); err != nil {
		os.RemoveAll(work)
		return "", "", fmt.Errorf("create installer input staging directory: %w", err)
	}
	if err = os.Mkdir(filepath.Join(staging, "soda"), 0o700); err != nil {
		os.RemoveAll(work)
		return "", "", fmt.Errorf("create installer input data directory: %w", err)
	}
	for _, name := range installerInputPaths() {
		if err = os.WriteFile(filepath.Join(staging, name), files[name], 0o600); err != nil {
			os.RemoveAll(work)
			return "", "", fmt.Errorf("stage installer input data: %w", err)
		}
	}
	return work, staging, nil
}

func (b *InstallerInputBuilder) runInstallerInputXorriso(ctx context.Context, staging, temporaryISO string) error {
	args := []string{"-as", "mkisofs", "-quiet", "-V", "OEMDRV", "-graft-points", "-o", temporaryISO}
	for _, name := range installerInputPaths() {
		args = append(args, name+"="+name)
	}
	if err := b.runner.Run(ctx, process.Command{Dir: staging, Name: "xorriso", Args: args}); err != nil {
		return fmt.Errorf("create installer input ISO: %w", err)
	}
	return nil
}

func publishInstallerInputISO(temporaryISO, outputAbsolute string) error {
	info, err := os.Lstat(temporaryISO)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("xorriso did not create a regular installer input ISO")
	}
	if err = os.Chmod(temporaryISO, 0o600); err != nil {
		return fmt.Errorf("protect installer input ISO: %w", err)
	}
	file, err := os.Open(temporaryISO)
	if err != nil {
		return fmt.Errorf("open installer input ISO: %w", err)
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync installer input ISO: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("close installer input ISO: %w", err)
	}
	if err = os.Link(temporaryISO, outputAbsolute); err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.New("installer input output already exists")
		}
		return fmt.Errorf("publish installer input ISO: %w", err)
	}
	published := true
	defer func() {
		if published {
			_ = os.Remove(outputAbsolute)
		}
	}()
	parent, err := os.Open(filepath.Dir(outputAbsolute))
	if err != nil {
		return fmt.Errorf("open installer input output directory: %w", err)
	}
	if err = parent.Sync(); err != nil {
		_ = parent.Close()
		return fmt.Errorf("sync installer input output directory: %w", err)
	}
	if err = parent.Close(); err != nil {
		return fmt.Errorf("close installer input output directory: %w", err)
	}
	published = false
	return nil
}

func (b *InstallerInputBuilder) verifyMedia(ctx context.Context, work, isoPath string, expected map[string][]byte) error {
	extracted := filepath.Join(work, "verified")
	if err := os.Mkdir(extracted, 0o700); err != nil {
		return fmt.Errorf("create installer input verification directory: %w", err)
	}
	if err := b.runner.Run(ctx, process.Command{
		Name: "xorriso",
		Args: []string{"-osirrox", "on", "-indev", isoPath, "-extract", "/", extracted},
	}); err != nil {
		return fmt.Errorf("inspect installer input ISO: %w", err)
	}

	actual := make(map[string][]byte, len(expected))
	err := filepath.WalkDir(extracted, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(extracted, path)
		if err != nil || relative == "." {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			if relative != "soda" {
				return fmt.Errorf("installer input ISO contains unexpected directory %s", relative)
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("installer input ISO contains non-regular path %s", relative)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		actual[relative] = contents
		return nil
	})
	if err != nil {
		return fmt.Errorf("verify installer input ISO contents: %w", err)
	}
	if len(actual) != len(expected) {
		return errors.New("installer input ISO does not contain exactly the required files")
	}
	for _, name := range installerInputPaths() {
		contents, present := actual[name]
		if !present || !bytes.Equal(contents, expected[name]) {
			return fmt.Errorf("installer input ISO contains unexpected data for %s", name)
		}
	}
	return nil
}

func installerInputPaths() []string {
	return []string{
		"ks.cfg",
		"soda/administrator-username",
		"soda/administrator-password",
		"soda/administrator-authorized-key",
		"soda/tailscale-auth-key",
	}
}
