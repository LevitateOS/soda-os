package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
)

const (
	administratorShell = "/bin/bash"
	workspaceGroup     = "soda-workspaces"
)

var administratorUsernamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

type accountsHost interface {
	UIDMin() (int, error)
	LookupGroup(context.Context, string) (linuxhost.Group, error)
	LookupAccount(context.Context, string) (linuxhost.Account, error)
	PasswordStatus(context.Context, linuxhost.Account) (linuxhost.PasswordStatus, error)
	ReadAuthorizedKeys(linuxhost.Account) ([]byte, error)
	InstallAuthorizedKeys(linuxhost.Account, []byte) error
}

type NativeAccounts struct {
	Host   accountsHost
	Runner linuxhost.CommandRunner
}

func (accounts NativeAccounts) dependencies() (accountsHost, linuxhost.CommandRunner) {
	runner := accounts.Runner
	if runner == nil {
		runner = linuxhost.ExecCommandRunner{}
	}
	host := accounts.Host
	if host == nil {
		native := linuxhost.NewNative()
		native.Runner = runner
		host = native
	}
	return host, runner
}

func (accounts NativeAccounts) Administrators(ctx context.Context) ([]Administrator, error) {
	host, _ := accounts.dependencies()
	wheel, err := host.LookupGroup(ctx, linuxhost.AdministratorGroup)
	if err != nil {
		return nil, err
	}
	members := make([]string, 0, len(wheel.Members))
	for member := range wheel.Members {
		if validateAdministratorUsername(member) != nil {
			return nil, errors.New("Linux wheel group contains an invalid member")
		}
		members = append(members, member)
	}
	sort.Strings(members)
	uidMin, err := host.UIDMin()
	if err != nil {
		return nil, err
	}
	administrators := make([]Administrator, 0, len(members))
	for _, username := range members {
		account, lookupErr := host.LookupAccount(ctx, username)
		if lookupErr != nil || !isAdministrator(account, uidMin) {
			continue
		}
		passwordSet := passwordIsSet(ctx, host, account)
		_, keyErr := host.ReadAuthorizedKeys(account)
		administrators = append(administrators, Administrator{
			Username: username, PasswordSet: passwordSet, SSHPublicKey: keyErr == nil,
		})
	}
	sort.Slice(administrators, func(i, j int) bool { return administrators[i].Username < administrators[j].Username })
	return administrators, nil
}

func passwordIsSet(ctx context.Context, host accountsHost, account linuxhost.Account) bool {
	status, err := host.PasswordStatus(ctx, account)
	return err == nil && status == linuxhost.PasswordSet
}

func (accounts NativeAccounts) Prepare(ctx context.Context, request AdministratorRequest) error {
	host, runner := accounts.dependencies()
	if _, err := preparePrimaryAccount(ctx, host, runner, request.Username, request.Password); err != nil {
		return err
	}
	if err := publishAdministratorKey(host, ctx, request.Username, []byte(request.AuthorizedKey)); err != nil {
		return fmt.Errorf("Linux account %s and its password were retained without the requested SSH key: %w", request.Username, err)
	}
	return nil
}

func (accounts NativeAccounts) Promote(ctx context.Context, username string) error {
	host, runner := accounts.dependencies()
	result, err := runner.Run(ctx, linuxhost.Command{Name: "/usr/sbin/usermod", Args: []string{"--append", "--groups", linuxhost.AdministratorGroup, "--", username}})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("promote Linux administrator %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	account, err := host.LookupAccount(ctx, username)
	if err != nil {
		return err
	}
	uidMin, err := host.UIDMin()
	if err != nil {
		return err
	}
	if !isAdministrator(account, uidMin) {
		return fmt.Errorf("Linux account %s is not an ordinary administrator after promotion", username)
	}
	return nil
}

func preparePrimaryAccount(ctx context.Context, host accountsHost, runner linuxhost.CommandRunner, username, password string) (linuxhost.Account, error) {
	if err := validateAdministratorUsername(username); err != nil {
		return linuxhost.Account{}, err
	}
	if err := validateAdministratorPassword(password); err != nil {
		return linuxhost.Account{}, err
	}
	uidMin, err := host.UIDMin()
	if err != nil {
		return linuxhost.Account{}, err
	}
	existing, found, err := existingPrimaryAccount(ctx, host, username, uidMin)
	if err != nil || found {
		return existing, err
	}
	if err = createInitialAdministrator(ctx, runner, username); err != nil {
		return linuxhost.Account{}, err
	}
	if err = setInitialAdministratorPassword(ctx, runner, username, password); err != nil {
		return linuxhost.Account{}, err
	}
	account, err := host.LookupAccount(ctx, username)
	if err != nil {
		return linuxhost.Account{}, fmt.Errorf("verify created primary account %s: %w", username, err)
	}
	if err = validateSupportedNewPrimary(account, username, uidMin); err != nil {
		return linuxhost.Account{}, err
	}
	return account, validatePasswordUsable(ctx, host, account)
}

func existingPrimaryAccount(ctx context.Context, host accountsHost, username string, uidMin int) (linuxhost.Account, bool, error) {
	account, err := host.LookupAccount(ctx, username)
	if errors.Is(err, linuxhost.ErrAccountNotFound) {
		return linuxhost.Account{}, false, nil
	}
	if err != nil {
		return linuxhost.Account{}, false, err
	}
	if err = validateSupportedNewPrimary(account, username, uidMin); err != nil {
		return linuxhost.Account{}, true, err
	}
	return account, true, validatePasswordUsable(ctx, host, account)
}

func createInitialAdministrator(ctx context.Context, runner linuxhost.CommandRunner, username string) error {
	result, err := runner.Run(ctx, linuxhost.Command{
		Name: "/usr/sbin/useradd",
		Args: []string{
			"--create-home", "--user-group", "--shell", administratorShell,
			"--home-dir", "/home/" + username, "--", username,
		},
	})
	if err != nil {
		return fmt.Errorf("create primary account %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("create primary account %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func setInitialAdministratorPassword(ctx context.Context, runner linuxhost.CommandRunner, username, password string) error {
	result, err := runner.Run(ctx, linuxhost.Command{
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

func publishAdministratorKey(host accountsHost, ctx context.Context, username string, authorizedKey []byte) error {
	if err := validateAdministratorUsername(username); err != nil {
		return err
	}
	uidMin, err := host.UIDMin()
	if err != nil {
		return err
	}
	account, err := host.LookupAccount(ctx, username)
	if err != nil {
		return err
	}
	if err = validateSupportedNewPrimary(account, username, uidMin); err != nil {
		return err
	}
	contents := append(bytes.TrimSpace(authorizedKey), '\n')
	if existing, readErr := host.ReadAuthorizedKeys(account); readErr == nil {
		if bytes.Equal(existing, contents) {
			return nil
		}
		return errors.New("refusing to overwrite a different authorized_keys file")
	}
	return host.InstallAuthorizedKeys(account, contents)
}

func validateAdministratorUsername(username string) error {
	if !administratorUsernamePattern.MatchString(username) {
		return errors.New("username must match [a-z][a-z0-9-]{0,23}")
	}
	return nil
}

func validateAdministratorPassword(password string) error {
	if password == "" || len(password) > 4096 {
		return errors.New("password must contain between 1 and 4096 bytes")
	}
	if strings.ContainsAny(password, "\x00\r\n") {
		return errors.New("password must not contain NUL, CR, or LF")
	}
	return nil
}

func validateSupportedNewPrimary(account linuxhost.Account, username string, uidMin int) error {
	if account.Username != username || !isPrimary(account, uidMin) || account.PrimaryGroup != username ||
		account.Home != "/home/"+username || account.Shell != administratorShell || account.HasGroup(linuxhost.AdministratorGroup) {
		return fmt.Errorf("Linux account %s does not have the supported ordinary primary-account shape", username)
	}
	return nil
}

func isPrimary(account linuxhost.Account, uidMin int) bool {
	return account.UID >= uidMin && validateAdministratorUsername(account.Username) == nil &&
		account.HasInteractiveShell() && !account.HasGroup(workspaceGroup)
}

func isAdministrator(account linuxhost.Account, uidMin int) bool {
	return isPrimary(account, uidMin) && account.HasGroup(linuxhost.AdministratorGroup)
}

func validatePasswordUsable(ctx context.Context, host accountsHost, account linuxhost.Account) error {
	status, err := host.PasswordStatus(ctx, account)
	if err != nil {
		return err
	}
	if status != linuxhost.PasswordSet {
		return fmt.Errorf("primary account %s does not have a usable Linux password", account.Username)
	}
	return nil
}
