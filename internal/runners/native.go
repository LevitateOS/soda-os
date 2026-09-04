package runners

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const defaultGitHubSource = "/usr/lib/soda/github-actions-runner"

type Command struct {
	Name        string
	Args        []string
	Directory   string
	Environment []string
	UID         uint32
	GID         uint32
}

type CommandResult struct {
	Stdout string
}

type CommandRunner interface {
	Run(context.Context, Command) (CommandResult, error)
	RunSecret(context.Context, Command, string) error
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, request Command) (CommandResult, error) {
	command := exec.CommandContext(ctx, request.Name, request.Args...)
	command.Dir = request.Directory
	command.Env = request.Environment
	if request.UID != 0 || request.GID != 0 {
		command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: request.UID, Gid: request.GID}}
	}
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return CommandResult{}, err
	}
	return CommandResult{Stdout: stdout.String()}, nil
}

type Native struct {
	RootPath     string
	LockPath     string
	GitHubSource string
	Runner       CommandRunner
}

func NewNative() *Native {
	return &Native{RootPath: DefaultRootPath, LockPath: DefaultLockPath, GitHubSource: defaultGitHubSource, Runner: ExecCommandRunner{}}
}

func (native *Native) List(ctx context.Context) ([]RunnerView, error) {
	entries, err := os.ReadDir(native.rootPath())
	if errors.Is(err, os.ErrNotExist) {
		return []RunnerView{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read local runner directory: %w", err)
	}
	views := make([]RunnerView, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || ValidateID(entry.Name()) != nil {
			continue
		}
		view, viewErr := native.runnerView(ctx, entry.Name())
		if viewErr != nil {
			return nil, viewErr
		}
		views = append(views, view)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ID < views[j].ID })
	return views, nil
}

func (native *Native) runnerView(ctx context.Context, id string) (RunnerView, error) {
	descriptor, err := native.readDescriptor(id)
	if err != nil {
		return RunnerView{}, err
	}
	service, err := native.serviceState(ctx, id)
	if err != nil {
		return RunnerView{}, err
	}
	version, err := native.providerVersion(ctx, descriptor)
	if err != nil {
		return RunnerView{}, err
	}
	return RunnerView{Descriptor: descriptor, Version: version, Capacity: RunnerCapacity, Service: service}, nil
}

func (native *Native) Start(ctx context.Context, id string) error {
	return native.serviceAction(ctx, id, "enable", "--now")
}

func (native *Native) Stop(ctx context.Context, id string) error {
	return native.serviceAction(ctx, id, "disable", "--now")
}

func (native *Native) Restart(ctx context.Context, id string) error {
	if err := native.serviceAction(ctx, id, "enable"); err != nil {
		return err
	}
	return native.serviceAction(ctx, id, "restart")
}

func (native *Native) Remove(ctx context.Context, id string) error {
	lock, err := native.lock()
	if err != nil {
		return err
	}
	defer lock.Close()
	descriptor, err := native.readDescriptor(id)
	if err != nil {
		return err
	}
	if _, err = native.run(ctx, "systemctl", "disable", "--now", native.unit(id)); err != nil {
		return errors.New("stop local runner listener")
	}
	if _, err = native.run(ctx, "userdel", descriptor.Account); err != nil {
		return errors.New("remove runner Linux account")
	}
	if err = os.RemoveAll(filepath.Join(native.rootPath(), id)); err != nil {
		return fmt.Errorf("remove runner state: %w", err)
	}
	return nil
}

func (native *Native) serviceAction(ctx context.Context, id, action string, options ...string) error {
	if _, err := native.readDescriptor(id); err != nil {
		return err
	}
	arguments := append([]string{action}, options...)
	if _, err := native.run(ctx, "systemctl", append(arguments, native.unit(id))...); err != nil {
		return fmt.Errorf("%s local runner listener", action)
	}
	return nil
}

func (native *Native) readDescriptor(id string) (Descriptor, error) {
	if err := ValidateID(id); err != nil {
		return Descriptor{}, err
	}
	contents, err := os.ReadFile(native.descriptorPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return Descriptor{}, fmt.Errorf("local runner %s does not exist", id)
	}
	if err != nil {
		return Descriptor{}, err
	}
	var descriptor Descriptor
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&descriptor); err != nil {
		return Descriptor{}, fmt.Errorf("decode local runner %s: %w", id, err)
	}
	account, accountErr := AccountName(id)
	if accountErr != nil || descriptor.ID != id || descriptor.Account != account || (descriptor.Provider != ProviderForgejo && descriptor.Provider != ProviderGitHub) {
		return Descriptor{}, fmt.Errorf("local runner %s descriptor is invalid", id)
	}
	return descriptor, nil
}

func (native *Native) writeDescriptor(descriptor Descriptor) error {
	path := native.descriptorPath(descriptor.ID)
	contents, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".descriptor-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if err = temporary.Chmod(0o644); err == nil {
		_, err = temporary.Write(append(contents, '\n'))
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary.Name(), path)
}

func (native *Native) serviceState(ctx context.Context, id string) (ServiceState, error) {
	result, err := native.run(ctx, "systemctl", "show", "--no-pager", "--property=LoadState,ActiveState,SubState,UnitFileState", native.unit(id))
	if err != nil {
		return ServiceState{}, fmt.Errorf("inspect local runner %s service: %w", id, err)
	}
	values := map[string]string{}
	for _, line := range strings.Split(result.Stdout, "\n") {
		key, value, found := strings.Cut(line, "=")
		if found {
			values[key] = value
		}
	}
	return ServiceState{Load: values["LoadState"], Active: values["ActiveState"], Sub: values["SubState"], Enabled: values["UnitFileState"]}, nil
}

func (native *Native) providerVersion(ctx context.Context, descriptor Descriptor) (string, error) {
	if descriptor.Provider == ProviderGitHub {
		app := filepath.Join(native.statePath(descriptor.ID), "actions-runner")
		result, err := native.runner().Run(ctx, Command{
			Name: "/usr/sbin/runuser", Directory: app,
			Args: []string{"--user", descriptor.Account, "--", filepath.Join(app, "bin", "Runner.Listener"), "--version"},
		})
		if err != nil {
			return "", fmt.Errorf("read GitHub runner %s client version: %w", descriptor.ID, err)
		}
		return strings.TrimSpace(result.Stdout), nil
	}
	result, err := native.run(ctx, "forgejo-runner", "--version")
	if err != nil {
		return "", errors.New("read Forgejo runner version")
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (native *Native) lock() (*os.File, error) {
	file, err := os.OpenFile(native.lockPath(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func (native *Native) run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	return native.runner().Run(ctx, Command{Name: name, Args: args})
}

func (native *Native) runner() CommandRunner {
	if native.Runner == nil {
		return ExecCommandRunner{}
	}
	return native.Runner
}

func (native *Native) rootPath() string {
	if native.RootPath != "" {
		return native.RootPath
	}
	return DefaultRootPath
}
func (native *Native) lockPath() string {
	if native.LockPath != "" {
		return native.LockPath
	}
	return DefaultLockPath
}
func (native *Native) githubSource() string {
	if native.GitHubSource != "" {
		return native.GitHubSource
	}
	return defaultGitHubSource
}
func (native *Native) statePath(id string) string {
	return filepath.Join(native.rootPath(), id, "state")
}
func (native *Native) descriptorPath(id string) string {
	return filepath.Join(native.rootPath(), id, "descriptor.json")
}
func (native *Native) unit(id string) string { return "soda-runner@" + id + ".service" }

func writeOwnedFile(path string, contents []byte, mode os.FileMode, owner identity) error {
	if err := os.WriteFile(path, contents, mode); err != nil {
		return err
	}
	return os.Chown(path, int(owner.UID), int(owner.GID))
}
