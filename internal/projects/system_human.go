package projects

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const maximumTeaConfigSize = 1 << 20

func (platform *NativePlatform) CreatePrimary(ctx context.Context, username, password string) (Account, error) {
	if err := ValidatePrimaryUsername(username); err != nil {
		return Account{}, err
	}
	if err := validateHumanPassword(password); err != nil {
		return Account{}, err
	}
	uidMin, err := platform.UIDMin()
	if err != nil {
		return Account{}, err
	}
	existing, found, err := platform.existingPrimary(ctx, username, uidMin)
	if err != nil || found {
		return existing, err
	}
	if err = platform.createPrimaryAccount(ctx, username); err != nil {
		return Account{}, err
	}
	if err = platform.setInitialPassword(ctx, username, password); err != nil {
		return Account{}, err
	}
	return platform.verifiedPrimary(ctx, username, uidMin)
}

func (platform *NativePlatform) existingPrimary(ctx context.Context, username string, uidMin int) (Account, bool, error) {
	account, err := platform.LookupAccount(ctx, username)
	if errors.Is(err, ErrAccountNotFound) {
		return Account{}, false, nil
	}
	if err != nil {
		return Account{}, false, err
	}
	if err = validateSupportedNewPrimary(account, username, uidMin); err != nil {
		return Account{}, true, err
	}
	return account, true, platform.validatePasswordUsable(ctx, account)
}

func (platform *NativePlatform) createPrimaryAccount(ctx context.Context, username string) error {
	result, err := platform.run(ctx, "/usr/sbin/useradd", "--create-home", "--user-group",
		"--shell", WorkspaceShell, "--home-dir", "/home/"+username, "--", username)
	if err != nil {
		return fmt.Errorf("create primary account %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("create primary account %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (platform *NativePlatform) setInitialPassword(ctx context.Context, username, password string) error {
	result, err := platform.runner().Run(ctx, Command{
		Name: "/usr/bin/passwd", Args: []string{"--stdin", "--", username}, Input: strings.NewReader(password + "\n"),
	})
	if err != nil {
		return fmt.Errorf("set initial Linux password for retained account %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("set initial Linux password for retained account %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (platform *NativePlatform) verifiedPrimary(ctx context.Context, username string, uidMin int) (Account, error) {
	account, err := platform.LookupAccount(ctx, username)
	if err != nil {
		return Account{}, fmt.Errorf("verify created primary account %s: %w", username, err)
	}
	if err = validateSupportedNewPrimary(account, username, uidMin); err != nil {
		return Account{}, err
	}
	if err = platform.validatePasswordUsable(ctx, account); err != nil {
		return Account{}, err
	}
	return account, nil
}

func validateSupportedNewPrimary(account Account, username string, uidMin int) error {
	if account.Username != username || !account.IsPrimary(uidMin) || account.PrimaryGroup != username ||
		account.Home != "/home/"+username || account.Shell != WorkspaceShell || account.Groups["wheel"] || account.Groups[WorkspaceGroup] {
		return fmt.Errorf("Linux account %s does not have the supported ordinary primary-account shape", username)
	}
	return nil
}

func (platform *NativePlatform) validatePasswordUsable(ctx context.Context, account Account) error {
	result, err := platform.run(ctx, "/usr/bin/passwd", "--status", account.Username)
	if err != nil {
		return fmt.Errorf("read Linux password status for %s: %w", account.Username, err)
	}
	fields := strings.Fields(result.Stdout)
	if result.ExitCode != 0 || len(fields) < 2 || fields[0] != account.Username || fields[1] != "P" {
		return fmt.Errorf("primary account %s does not have a usable Linux password", account.Username)
	}
	return nil
}

func (platform *NativePlatform) PublishHuman(ctx context.Context, actor Account, username string, authorizedKey []byte) error {
	if err := ValidatePrimaryUsername(username); err != nil {
		return err
	}
	uidMin, err := platform.UIDMin()
	if err != nil {
		return err
	}
	target, err := platform.LookupAccount(ctx, username)
	if err != nil {
		return err
	}
	if err = validateSupportedNewPrimary(target, username, uidMin); err != nil {
		return err
	}
	contents, err := platform.readStagedHumanTea(actor, username)
	if err != nil {
		return err
	}
	if err = platform.installTeaConfig(ctx, target, contents); err != nil {
		return err
	}
	keyContents := append(bytes.TrimSpace(authorizedKey), '\n')
	if err = platform.installAuthorizedKeysIdempotent(target, keyContents); err != nil {
		return err
	}
	return nil
}

func (platform *NativePlatform) readStagedHumanTea(actor Account, username string) ([]byte, error) {
	file, err := platform.openStagedHumanTea(actor, username)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := descriptorStat(file)
	if err != nil || stat.Mode&0o777 != 0o600 {
		return nil, errors.New("staged Tea configuration must have mode 0600")
	}
	contents, err := io.ReadAll(io.LimitReader(file, maximumTeaConfigSize+1))
	if err != nil || len(contents) == 0 || len(contents) > maximumTeaConfigSize {
		return nil, errors.New("staged Tea configuration has an invalid size")
	}
	return contents, nil
}

func (platform *NativePlatform) openStagedHumanTea(actor Account, username string) (*os.File, error) {
	people, exists, err := platform.openPeopleStaging(actor)
	if err != nil || !exists {
		return nil, errors.New("protected Tea staging does not exist")
	}
	defer people.Close()
	target, err := openOwnedDirectoryChain(people, []string{username}, actor.UID)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	if err = requireDirectoryNames(target, "config", "home"); err != nil {
		return nil, err
	}
	if err = validateEmptyStagedHome(target, actor.UID); err != nil {
		return nil, err
	}
	return openStagedTeaConfig(target, actor.UID)
}

func validateEmptyStagedHome(target *os.File, uid int) error {
	home, err := openOwnedDirectoryChain(target, []string{"home"}, uid)
	if err != nil {
		return err
	}
	defer home.Close()
	return requireDirectoryNames(home)
}

func openStagedTeaConfig(target *os.File, uid int) (*os.File, error) {
	config, err := openOwnedDirectoryChain(target, []string{"config", "tea"}, uid)
	if err != nil {
		return nil, err
	}
	defer config.Close()
	if err = requireDirectoryNames(config, "config.yml"); err != nil {
		return nil, err
	}
	return openOwnedRegularAt(config, "config.yml", uid, "staged Tea configuration")
}

func requireDirectoryNames(directory *os.File, expected ...string) error {
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}
	actual := make(map[string]bool, len(entries))
	for _, entry := range entries {
		actual[entry.Name()] = true
	}
	if len(actual) != len(expected) {
		return errors.New("staging directory contains unexpected entries")
	}
	for _, name := range expected {
		if !actual[name] {
			return errors.New("staging directory is incomplete")
		}
	}
	return nil
}

func openOwnedRegularAt(parent *os.File, name string, uid int, description string) (*os.File, error) {
	fd, err := unix.Openat2(int(parent.Fd()), name, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		_ = unix.Close(fd)
		return nil, errors.New("open regular file descriptor")
	}
	if err = validateOwnedRegularFile(file, uid, description); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func (platform *NativePlatform) installTeaConfig(ctx context.Context, account Account, contents []byte) error {
	home, err := platform.openValidatedAccountHome(account)
	if err != nil {
		return err
	}
	defer home.Close()
	config, err := ensureOwnedDirectoryAt(home, ".config", account)
	if err != nil {
		return err
	}
	defer config.Close()
	teaDirectory, err := ensureOwnedDirectoryAt(config, "tea", account)
	if err != nil {
		return err
	}
	defer teaDirectory.Close()
	exists, err := validateExistingTeaConfig(teaDirectory, account, contents)
	if err != nil {
		return err
	}
	if exists {
		return platform.relabelTeaConfig(ctx, account)
	}
	if err = writeOwnedFileAt(teaDirectory, ".soda-config.tmp", "config.yml", account, contents); err != nil {
		return err
	}
	return platform.relabelTeaConfig(ctx, account)
}

func validateExistingTeaConfig(directory *os.File, account Account, expected []byte) (bool, error) {
	existing, err := openOwnedRegularAt(directory, "config.yml", account.UID, "Tea configuration")
	if isMissing(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer existing.Close()
	stat, err := descriptorStat(existing)
	if err != nil || stat.Mode&0o777 != 0o600 {
		return true, errors.New("Tea configuration must have mode 0600")
	}
	current, err := io.ReadAll(io.LimitReader(existing, maximumTeaConfigSize+1))
	if err != nil || !bytes.Equal(current, expected) {
		return true, errors.New("refusing to overwrite a different Tea configuration")
	}
	return true, nil
}

func (platform *NativePlatform) relabelTeaConfig(ctx context.Context, account Account) error {
	result, err := platform.run(ctx, "/usr/sbin/restorecon", "-R", filepath.Join(account.Home, ".config", "tea"))
	if err != nil || result.ExitCode != 0 {
		return fmt.Errorf("relabel Tea configuration: %s", strings.TrimSpace(result.Stderr))
	}
	return nil
}

func writeOwnedFileAt(parent *os.File, temporary, final string, account Account, contents []byte) (returnErr error) {
	fd, err := unix.Openat2(int(parent.Fd()), temporary, &unix.OpenHow{
		Flags: unix.O_CREAT | unix.O_EXCL | unix.O_WRONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Mode:  0o600, Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), temporary)
	defer file.Close()
	published := false
	defer func() {
		if !published {
			returnErr = errors.Join(returnErr, unix.Unlinkat(int(parent.Fd()), temporary, 0))
		}
	}()
	if _, err = file.Write(contents); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = unix.Fchown(fd, account.UID, account.GID); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = unix.Renameat2(int(parent.Fd()), temporary, int(parent.Fd()), final, unix.RENAME_NOREPLACE); err != nil {
		return err
	}
	published = true
	return unix.Fsync(int(parent.Fd()))
}

func (platform *NativePlatform) installAuthorizedKeysIdempotent(account Account, contents []byte) error {
	if existing, err := platform.ReadAuthorizedKeys(account); err == nil {
		if bytes.Equal(existing, contents) {
			return nil
		}
		return errors.New("refusing to overwrite a different authorized_keys file")
	}
	return platform.InstallAuthorizedKeys(account, contents)
}

func (platform *NativePlatform) InstallWorkspaceTea(primary, workspace Account) error {
	source, err := platform.readTeaConfig(primary)
	if err != nil {
		return fmt.Errorf("read primary Tea configuration: %w", err)
	}
	return platform.installTeaConfig(context.Background(), workspace, source)
}

func (platform *NativePlatform) readTeaConfig(account Account) ([]byte, error) {
	home, err := platform.openValidatedAccountHome(account)
	if err != nil {
		return nil, err
	}
	defer home.Close()
	config, err := openOwnedDirectoryChain(home, []string{".config", "tea"}, account.UID)
	if err != nil {
		return nil, err
	}
	defer config.Close()
	file, err := openOwnedRegularAt(config, "config.yml", account.UID, "Tea configuration")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := descriptorStat(file)
	if err != nil || stat.Mode&0o777 != 0o600 {
		return nil, errors.New("Tea configuration must have mode 0600")
	}
	contents, err := io.ReadAll(io.LimitReader(file, maximumTeaConfigSize+1))
	if err != nil || len(contents) == 0 || len(contents) > maximumTeaConfigSize {
		return nil, errors.New("Tea configuration has an invalid size")
	}
	return contents, nil
}
