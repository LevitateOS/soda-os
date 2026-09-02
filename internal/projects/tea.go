package projects

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultTeaPath = "/usr/bin/tea"
	teaLoginName   = "soda"
	teaTokenName   = "soda-os-tea"
	teaScopes      = "read:user,write:repository,write:issue"
)

type NativeTea struct {
	Platform *NativePlatform
	Runner   CommandRunner
	Binary   string
}

func NewNativeTea(platform *NativePlatform) NativeTea {
	return NativeTea{Platform: platform, Runner: platform.runner(), Binary: defaultTeaPath}
}

func (tea NativeTea) Preflight(actor Account, username string) error {
	if err := ValidatePrimaryUsername(username); err != nil {
		return err
	}
	info, err := os.Stat(tea.binary())
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return errors.New("Tea login boundary is unavailable")
	}
	people, exists, err := tea.Platform.openPeopleStaging(actor)
	if err != nil || !exists {
		return err
	}
	defer people.Close()
	target, exists, err := openOptionalOwnedDirectory(people, username, actor.UID, "human onboarding staging directory")
	if err != nil || !exists {
		return err
	}
	defer target.Close()
	return validateHumanTeaStaging(target, actor.UID)
}

func validateHumanTeaStaging(target *os.File, uid int) error {
	if err := requireDirectoryNames(target, "config", "home"); err != nil {
		return err
	}
	if err := validateEmptyStagedHome(target, uid); err != nil {
		return err
	}
	config, err := openOwnedDirectoryChain(target, []string{"config"}, uid)
	if err != nil {
		return err
	}
	defer config.Close()
	return validateStagedTeaConfigDirectory(config, uid)
}

func validateStagedTeaConfigDirectory(config *os.File, uid int) error {
	entries, err := config.ReadDir(-1)
	if err != nil || len(entries) == 0 {
		return err
	}
	if len(entries) != 1 || entries[0].Name() != "tea" || !entries[0].IsDir() {
		return errors.New("protected Tea staging contains unexpected state")
	}
	teaDirectory, err := openOwnedDirectoryChain(config, []string{"tea"}, uid)
	if err != nil {
		return err
	}
	defer teaDirectory.Close()
	if err = requireDirectoryNames(teaDirectory, "config.yml"); err != nil {
		return err
	}
	file, err := openOwnedRegularAt(teaDirectory, "config.yml", uid, "staged Tea configuration")
	if err != nil {
		return err
	}
	defer file.Close()
	return validateStagedTeaConfigFile(file)
}

func validateStagedTeaConfigFile(file *os.File) error {
	stat, err := descriptorStat(file)
	if err != nil || stat.Mode&0o777 != 0o600 {
		return errors.New("staged Tea configuration must have mode 0600")
	}
	return nil
}

func (tea NativeTea) StageLogin(ctx context.Context, actor Account, username, forgejoURL, password string) error {
	if err := ValidatePrimaryUsername(username); err != nil {
		return err
	}
	if err := validateHumanPassword(password); err != nil {
		return err
	}
	home, config, err := tea.prepareStaging(actor, username)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(filepath.Join(config, "tea", "config.yml")); err == nil {
		return tea.VerifyLogin(ctx, actor, username)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect staged Tea configuration: %w", err)
	}
	result, err := tea.runner().Run(ctx, Command{
		Name: tea.binary(),
		Args: []string{"logins", "add", "--name", teaLoginName, "--url", forgejoURL,
			"--user", username, "--password-stdin", "--token-name", teaTokenName, "--scopes", teaScopes},
		Input:       strings.NewReader(password),
		Environment: tea.environment(home, config, username),
	})
	if err != nil {
		return fmt.Errorf("run Tea login: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("Tea login failed: %s", strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (tea NativeTea) VerifyLogin(ctx context.Context, actor Account, username string) error {
	home, config := tea.stagingPaths(actor, username)
	if actor.Username == username {
		home, config = actor.Home, filepath.Join(actor.Home, ".config")
	}
	result, err := tea.runner().Run(ctx, Command{
		Name: tea.binary(), Args: []string{"api", "--login", teaLoginName, "user"},
		Environment: tea.environment(home, config, username),
	})
	if err != nil {
		return fmt.Errorf("verify Tea login: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("verify Tea login: %s", strings.TrimSpace(result.Stderr))
	}
	var identity struct {
		Login string `json:"login"`
	}
	decoder := json.NewDecoder(bytes.NewBufferString(result.Stdout))
	if err = decoder.Decode(&identity); err != nil || identity.Login != username {
		return errors.New("Tea login does not identify the expected Linux user")
	}
	return nil
}

func (tea NativeTea) CleanupStaging(actor Account, username string) error {
	if err := ValidatePrimaryUsername(username); err != nil {
		return err
	}
	people, exists, err := tea.Platform.openPeopleStaging(actor)
	if err != nil || !exists {
		return err
	}
	defer people.Close()
	return removeOwnedDirectoryAt(people, username, actor.UID)
}

func (tea NativeTea) prepareStaging(actor Account, username string) (string, string, error) {
	people, err := tea.Platform.ensurePeopleStaging(actor)
	if err != nil {
		return "", "", err
	}
	defer people.Close()
	target, err := ensureCallerOwnedDirectoryAt(people, username, actor, "human onboarding staging directory")
	if err != nil {
		return "", "", err
	}
	defer target.Close()
	for _, name := range []string{"home", "config"} {
		directory, directoryErr := ensureCallerOwnedDirectoryAt(target, name, actor, "Tea staging "+name)
		if directoryErr != nil {
			return "", "", directoryErr
		}
		directory.Close()
	}
	home, config := tea.stagingPaths(actor, username)
	return home, config, nil
}

func (tea NativeTea) stagingPaths(actor Account, username string) (string, string) {
	base := filepath.Join(tea.Platform.RuntimeRoot, fmt.Sprint(actor.UID), "soda-projects", "people", username)
	return filepath.Join(base, "home"), filepath.Join(base, "config")
}

func (tea NativeTea) environment(home, config, username string) []string {
	return []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + config,
		"USER=" + username,
		"LOGNAME=" + username,
		"PATH=/usr/bin:/bin",
		"LANG=C.UTF-8",
	}
}

func (tea NativeTea) binary() string {
	if tea.Binary != "" {
		return tea.Binary
	}
	return defaultTeaPath
}

func (tea NativeTea) runner() CommandRunner {
	if tea.Runner != nil {
		return tea.Runner
	}
	return tea.Platform.runner()
}
