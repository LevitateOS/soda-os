package setup

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LevitateOS/soda-os/internal/projects"
)

type NativeAccounts struct {
	Platform *projects.NativePlatform
	Runner   projects.CommandRunner
}

func (accounts NativeAccounts) dependencies() (*projects.NativePlatform, projects.CommandRunner) {
	platform := accounts.Platform
	if platform == nil {
		platform = projects.NewNativePlatform()
	}
	runner := accounts.Runner
	if runner == nil {
		runner = platform.Runner
	}
	if runner == nil {
		runner = projects.ExecCommandRunner{}
	}
	return platform, runner
}

func (accounts NativeAccounts) Administrators(ctx context.Context) ([]Administrator, error) {
	platform, runner := accounts.dependencies()
	result, err := runner.Run(ctx, projects.Command{Name: "/usr/bin/getent", Args: []string{"group", "wheel"}})
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, errors.New("Linux wheel group is unavailable")
	}
	members, err := wheelMembers(result.Stdout)
	if err != nil {
		return nil, err
	}
	uidMin, err := platform.UIDMin()
	if err != nil {
		return nil, err
	}
	administrators := make([]Administrator, 0, len(members))
	for _, username := range members {
		account, lookupErr := platform.LookupAccount(ctx, username)
		if lookupErr != nil || !account.IsAdministrator(uidMin) {
			continue
		}
		passwordSet := passwordIsSet(ctx, runner, username)
		_, keyErr := platform.ReadAuthorizedKeys(account)
		administrators = append(administrators, Administrator{
			Username: username, PasswordSet: passwordSet, SSHPublicKey: keyErr == nil,
		})
	}
	sort.Slice(administrators, func(i, j int) bool { return administrators[i].Username < administrators[j].Username })
	return administrators, nil
}

func wheelMembers(record string) ([]string, error) {
	fields := strings.Split(strings.TrimSpace(record), ":")
	if len(fields) != 4 || fields[0] != "wheel" {
		return nil, errors.New("Linux wheel group record is invalid")
	}
	if fields[3] == "" {
		return nil, nil
	}
	members := strings.Split(fields[3], ",")
	for _, member := range members {
		if projects.ValidatePrimaryUsername(member) != nil {
			return nil, errors.New("Linux wheel group contains an invalid member")
		}
	}
	return members, nil
}

func passwordIsSet(ctx context.Context, runner projects.CommandRunner, username string) bool {
	result, err := runner.Run(ctx, projects.Command{Name: "/usr/bin/passwd", Args: []string{"--status", username}})
	if err != nil || result.ExitCode != 0 {
		return false
	}
	fields := strings.Fields(result.Stdout)
	return len(fields) >= 2 && fields[0] == username && fields[1] == "P"
}

func (accounts NativeAccounts) Prepare(ctx context.Context, request AdministratorRequest) error {
	platform, _ := accounts.dependencies()
	if _, err := platform.CreatePrimary(ctx, request.Username, request.Password); err != nil {
		return err
	}
	if err := platform.PublishHuman(ctx, request.Username, []byte(request.AuthorizedKey)); err != nil {
		return fmt.Errorf("Linux account %s and its password were retained without the requested SSH key: %w", request.Username, err)
	}
	return nil
}

func (accounts NativeAccounts) Promote(ctx context.Context, username string) error {
	platform, runner := accounts.dependencies()
	result, err := runner.Run(ctx, projects.Command{Name: "/usr/sbin/usermod", Args: []string{"--append", "--groups", "wheel", "--", username}})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("promote Linux administrator %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	account, err := platform.LookupAccount(ctx, username)
	if err != nil {
		return err
	}
	uidMin, err := platform.UIDMin()
	if err != nil {
		return err
	}
	if !account.IsAdministrator(uidMin) {
		return fmt.Errorf("Linux account %s is not an ordinary administrator after promotion", username)
	}
	return nil
}
