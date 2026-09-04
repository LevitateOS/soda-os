package runners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const githubRegistrationTimeout = 2 * time.Minute

type preparedRunner struct {
	account string
	state   string
	owner   identity
}

type identity struct{ UID, GID uint32 }

func (native *Native) Create(ctx context.Context, request CreateRequest) error {
	lock, err := native.lock()
	if err != nil {
		return err
	}
	defer lock.Close()
	prepared, err := native.prepareAccount(ctx, request.ID)
	if err != nil {
		return err
	}
	return native.registerPrepared(ctx, prepared, request)
}

func (native *Native) registerPrepared(ctx context.Context, prepared preparedRunner, request CreateRequest) (returnErr error) {
	retained := false
	defer func() {
		if !retained {
			returnErr = native.creationFailure(prepared, returnErr)
		}
	}()
	if err := native.configureProvider(ctx, prepared, request); err != nil {
		return err
	}
	if err := native.recordRunner(prepared.account, request); err != nil {
		if request.Provider == ProviderGitHub {
			return fmt.Errorf("GitHub registration completed, but local runner details could not be saved; inspect/remove the GitHub runner record before retrying: %w", err)
		}
		return fmt.Errorf("save local runner details: %w", err)
	}
	retained = true
	if _, err := native.run(ctx, "systemctl", "enable", "--now", native.unit(request.ID)); err != nil {
		return fmt.Errorf("runner was registered and retained, but its local listener did not start: %w", err)
	}
	return nil
}

func (native *Native) prepareAccount(ctx context.Context, id string) (_ preparedRunner, returnErr error) {
	if err := native.requireAvailable(id); err != nil {
		return preparedRunner{}, err
	}
	account, _ := AccountName(id)
	state := native.statePath(id)
	if err := os.MkdirAll(filepath.Dir(state), 0o755); err != nil {
		return preparedRunner{}, err
	}
	args := []string{"--system", "--gid", RunnerGroup, "--home-dir", state, "--no-create-home", "--shell", RunnerShell, "--comment", "soda-runner=" + id, account}
	if _, err := native.run(ctx, "useradd", args...); err != nil {
		return preparedRunner{}, errors.New("create runner Linux account")
	}
	prepared := preparedRunner{account: account, state: state}
	retained := false
	defer func() {
		if !retained {
			returnErr = native.creationFailure(prepared, returnErr)
		}
	}()
	owner, err := lookupIdentity(account)
	if err != nil {
		return prepared, err
	}
	if err = createOwnedDirectory(state, owner); err != nil {
		return prepared, err
	}
	prepared.owner = owner
	retained = true
	return prepared, nil
}

func (native *Native) requireAvailable(id string) error {
	if _, err := os.Stat(native.descriptorPath(id)); err == nil {
		return fmt.Errorf("local runner %s already exists", id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	account, _ := AccountName(id)
	if _, err := user.Lookup(account); err == nil {
		return fmt.Errorf("Linux account %s already exists", account)
	} else if _, ok := err.(user.UnknownUserError); !ok {
		return fmt.Errorf("look up runner account: %w", err)
	}
	return nil
}

func (native *Native) creationFailure(prepared preparedRunner, cause error) error {
	if _, err := native.run(context.Background(), "userdel", prepared.account); err != nil {
		return errors.Join(cause, fmt.Errorf("remove local runner account %s: %w; account removal is unconfirmed and state remains at %s", prepared.account, err, prepared.state))
	}
	if err := os.RemoveAll(filepath.Dir(prepared.state)); err != nil {
		return errors.Join(cause, fmt.Errorf("local runner account %s was removed, but state at %s was not fully removed: %w", prepared.account, prepared.state, err))
	}
	return fmt.Errorf("%w; local runner account %s and state at %s were removed", cause, prepared.account, prepared.state)
}

func (native *Native) configureProvider(ctx context.Context, prepared preparedRunner, request CreateRequest) error {
	if request.Provider == ProviderForgejo {
		return native.configureForgejo(prepared.state, prepared.owner, request)
	}
	return native.configureGitHub(ctx, prepared.state, prepared.owner, request)
}

func (native *Native) recordRunner(account string, request CreateRequest) error {
	architecture, err := NativeArchitecture()
	if err != nil {
		return err
	}
	return native.writeDescriptor(Descriptor{ID: request.ID, Provider: request.Provider, RegistrationURL: request.RegistrationURL, Account: account, Architecture: architecture})
}

func lookupIdentity(name string) (identity, error) {
	account, err := user.Lookup(name)
	if err != nil {
		return identity{}, fmt.Errorf("look up new runner account: %w", err)
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil {
		return identity{}, errors.New("runner account has an invalid UID")
	}
	gid, err := strconv.ParseUint(account.Gid, 10, 32)
	if err != nil {
		return identity{}, errors.New("runner account has an invalid GID")
	}
	return identity{uint32(uid), uint32(gid)}, nil
}

func createOwnedDirectory(path string, owner identity) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chown(path, int(owner.UID), int(owner.GID))
}

func (native *Native) configureForgejo(state string, owner identity, request CreateRequest) error {
	tokenPath := filepath.Join(state, "forgejo-token")
	if err := writeOwnedFile(tokenPath, []byte(request.RegistrationToken+"\n"), 0o600, owner); err != nil {
		return fmt.Errorf("write provider-owned Forgejo connection token: %w", err)
	}
	contents, err := forgejoConfiguration(state, tokenPath, request)
	if err != nil {
		return err
	}
	if err = writeOwnedFile(filepath.Join(state, "forgejo-runner.yml"), contents, 0o600, owner); err != nil {
		return fmt.Errorf("write provider-owned Forgejo runner configuration: %w", err)
	}
	return createOwnedDirectory(filepath.Join(state, "work"), owner)
}

func forgejoConfiguration(state, tokenPath string, request CreateRequest) ([]byte, error) {
	labels := strings.Split(request.Labels, ",")
	configuration := map[string]any{
		"log":       map[string]any{"level": "info", "job_level": "info"},
		"runner":    map[string]any{"file": ".runner", "capacity": RunnerCapacity, "labels": labels, "shutdown_timeout": "30s"},
		"cache":     map[string]any{"enabled": false},
		"container": map[string]any{"docker_host": "-", "valid_volumes": []string{}},
		"host":      map[string]any{"workdir_parent": filepath.Join(state, "work")},
		"server": map[string]any{"connections": map[string]any{"forgejo": map[string]any{
			"url": request.RegistrationURL, "uuid": request.RegistrationID, "token_url": "file:" + tokenPath, "labels": labels,
		}}},
	}
	contents, err := json.MarshalIndent(configuration, "", "  ")
	return append(contents, '\n'), err
}

func (native *Native) configureGitHub(ctx context.Context, state string, owner identity, request CreateRequest) error {
	app, err := native.copyGitHubRunner(ctx, state, owner)
	if err != nil {
		return err
	}
	command := githubRegistrationCommand(app, state, owner, request)
	if err = native.runGitHubRegistration(ctx, command, request.RegistrationToken); err != nil {
		return errors.New("GitHub registration failed; inspect/remove any GitHub runner record before retrying")
	}
	if _, err = os.Stat(filepath.Join(app, ".runner")); err != nil {
		return errors.New("GitHub registration completed without native runner state; inspect/remove the GitHub runner record before retrying")
	}
	return nil
}

func (native *Native) runGitHubRegistration(ctx context.Context, command Command, token string) error {
	registrationContext, cancel := context.WithTimeout(ctx, githubRegistrationTimeout)
	defer cancel()
	return native.runner().RunSecret(registrationContext, command, token)
}

func (native *Native) copyGitHubRunner(ctx context.Context, state string, owner identity) (string, error) {
	app := filepath.Join(state, "actions-runner")
	if err := os.Mkdir(app, 0o755); err != nil {
		return "", err
	}
	if _, err := native.run(ctx, "cp", "--archive", "--reflink=auto", native.githubSource()+"/.", app); err != nil {
		return "", errors.New("copy pinned GitHub runner")
	}
	if _, err := native.run(ctx, "restorecon", "-RF", app); err != nil {
		return "", errors.New("label pinned GitHub runner")
	}
	if err := os.Chown(app, int(owner.UID), int(owner.GID)); err != nil {
		return "", err
	}
	for _, name := range []string{"_diag", "_work"} {
		if err := createOwnedDirectory(filepath.Join(app, name), owner); err != nil {
			return "", err
		}
	}
	return app, nil
}

func githubRegistrationCommand(app, state string, owner identity, request CreateRequest) Command {
	args := []string{"--url", request.RegistrationURL, "--name", request.ID, "--runnergroup", "default", "--work", "_work", "--disableupdate", "--labels", request.Labels}
	return Command{Name: filepath.Join(app, "config.sh"), Args: args, Directory: app, Environment: []string{"HOME=" + state, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin"}, UID: owner.UID, GID: owner.GID}
}
