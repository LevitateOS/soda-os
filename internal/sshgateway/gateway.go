package sshgateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LevitateOS/soda-os/internal/domain"
	"golang.org/x/sys/unix"
)

const (
	defaultProjectsRoot = "/srv/soda/projects"
	sftpServer          = "/usr/libexec/openssh/sftp-server"
)

type Options struct {
	Actor           string
	Project         string
	Worktree        string
	Home            string
	ProjectsRoot    string
	OriginalCommand string
	Shell           string
	Environment     []string
}

type Invocation struct {
	Path   string
	Argv   []string
	Env    []string
	Dir    string
	Banner string
}

type sessionLayout struct {
	projectRoot string
	worktree    string
	home        string
}

type Executor interface {
	Exec(Invocation) error
}

type UnixExecutor struct{}

func (UnixExecutor) Exec(invocation Invocation) error {
	if err := os.Chdir(invocation.Dir); err != nil {
		return fmt.Errorf("change to Soda worktree: %w", err)
	}
	if invocation.Banner != "" {
		if _, err := fmt.Fprint(os.Stdout, invocation.Banner); err != nil {
			return fmt.Errorf("write Soda session banner: %w", err)
		}
	}
	return unix.Exec(invocation.Path, invocation.Argv, invocation.Env)
}

func Run(options Options, executor Executor) error {
	invocation, err := BuildInvocation(options)
	if err != nil {
		return err
	}
	if executor == nil {
		return errors.New("Soda SSH executor is unavailable")
	}
	if err := executor.Exec(invocation); err != nil {
		return fmt.Errorf("failed to enter Soda project session: %w", err)
	}
	return nil
}

func BuildInvocation(options Options) (Invocation, error) {
	if err := validateActor(options.Actor); err != nil {
		return Invocation{}, err
	}
	if err := validateActor(options.Project); err != nil {
		return Invocation{}, errors.New("invalid Soda project")
	}

	layout, err := resolveSessionLayout(options)
	if err != nil {
		return Invocation{}, err
	}
	if err := ensureGitWorktree(layout.worktree); err != nil {
		return Invocation{}, err
	}
	path, argv, err := sessionCommand(options.OriginalCommand, options.Shell)
	if err != nil {
		return Invocation{}, err
	}
	environment := mergeEnvironment(options.Environment, map[string]string{
		"HOME":            layout.home,
		"PWD":             layout.worktree,
		"SODA_ACTOR":      options.Actor,
		"SODA_PROJECT":    options.Project,
		"SODA_WORKTREE":   layout.worktree,
		"XDG_CACHE_HOME":  filepath.Join(layout.home, ".cache"),
		"XDG_CONFIG_HOME": filepath.Join(layout.home, ".config"),
		"XDG_DATA_HOME":   filepath.Join(layout.home, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(layout.home, ".local", "state"),
	})

	profile, err := projectEnvironment(layout.projectRoot)
	if err != nil {
		return Invocation{}, err
	}
	if inherited := environmentValue(environment, "PATH"); inherited != "" {
		profile["PATH"] += ":" + inherited
	}
	environment = mergeEnvironment(environment, profile)

	banner := ""
	if options.OriginalCommand == "" && environmentValue(options.Environment, "SSH_TTY") != "" {
		banner = fmt.Sprintf("Soda OS\nPerson: %s\nProject: %s\nBranch: people/%s\nWorkspace: %s\n\n", options.Actor, options.Project, options.Actor, layout.worktree)
	}
	return Invocation{Path: path, Argv: argv, Env: environment, Dir: layout.worktree, Banner: banner}, nil
}

func resolveSessionLayout(options Options) (sessionLayout, error) {
	projectsRoot := options.ProjectsRoot
	if projectsRoot == "" {
		projectsRoot = defaultProjectsRoot
	}
	root, err := canonicalDirectory(projectsRoot, "projects root")
	if err != nil {
		return sessionLayout{}, err
	}
	worktree, err := canonicalDirectory(options.Worktree, "worktree")
	if err != nil || !containedBy(root, worktree) {
		if err != nil {
			return sessionLayout{}, err
		}
		return sessionLayout{}, errors.New("worktree is outside the Soda projects root")
	}
	projectRoot, err := canonicalDirectory(filepath.Join(root, options.Project), "project root")
	if err != nil {
		return sessionLayout{}, err
	}
	if worktree != filepath.Join(projectRoot, "worktrees", options.Actor) {
		return sessionLayout{}, errors.New("worktree does not match the Soda actor and project")
	}
	home, err := canonicalDirectory(options.Home, "session home")
	if err != nil {
		return sessionLayout{}, err
	}
	if home != filepath.Join(projectRoot, ".soda", "people", options.Actor, "home") {
		return sessionLayout{}, errors.New("session home does not match the Soda actor and project")
	}
	return sessionLayout{projectRoot: projectRoot, worktree: worktree, home: home}, nil
}

func ensureGitWorktree(worktree string) error {
	if _, err := os.Stat(filepath.Join(worktree, ".git")); err != nil {
		if os.IsNotExist(err) {
			return errors.New("worktree is not a Git worktree")
		}
		return fmt.Errorf("inspect worktree Git metadata: %w", err)
	}
	return nil
}

func validateActor(actor string) error {
	if actor == "" {
		return errors.New("invalid Soda actor")
	}
	for _, character := range actor {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return errors.New("invalid Soda actor")
		}
	}
	return nil
}

func canonicalDirectory(path, label string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s is unavailable", label)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("%s %s is unavailable: %w", label, path, err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", fmt.Errorf("resolve %s %s: %w", label, path, err)
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return "", fmt.Errorf("%s %s is unavailable: %w", label, path, err)
	}
	return filepath.Clean(canonical), nil
}

func containedBy(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func sessionCommand(original, shell string) (string, []string, error) {
	switch original {
	case "internal-sftp":
		return sftpServer, []string{sftpServer}, nil
	case "":
		if shell == "" {
			shell = "/bin/bash"
		}
		resolved, err := exec.LookPath(shell)
		if err != nil {
			return "", nil, fmt.Errorf("login shell %s is unavailable: %w", shell, err)
		}
		return resolved, []string{resolved, "-l"}, nil
	default:
		return "/bin/bash", []string{"/bin/bash", "-lc", original}, nil
	}
}

func projectEnvironment(projectRoot string) (map[string]string, error) {
	path := filepath.Join(projectRoot, ".soda", "environment.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read project environment: %w", err)
	}
	var value domain.ProjectEnvironment
	if err := json.Unmarshal(contents, &value); err != nil {
		return nil, fmt.Errorf("parse project environment: %w", err)
	}
	pathValue, err := environmentPath(value.Path)
	if err != nil {
		return nil, err
	}
	if value.Profile == "" || strings.ContainsAny(value.Profile, "\x00\r\n") {
		return nil, errors.New("project environment profile is invalid")
	}
	variables, err := environmentVariables(value.Variables)
	if err != nil {
		return nil, err
	}
	variables["SODA_PROFILE"] = value.Profile
	variables["PATH"] = pathValue
	return variables, nil
}

func environmentPath(entries []string) (string, error) {
	if len(entries) == 0 {
		return "", errors.New("project environment PATH is empty")
	}
	for _, entry := range entries {
		if entry == "" || !filepath.IsAbs(entry) || strings.ContainsAny(entry, "\x00\r\n:") {
			return "", errors.New("project environment PATH entry is invalid")
		}
	}
	return strings.Join(entries, ":"), nil
}

func environmentVariables(variables map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(variables)+2)
	for name, value := range variables {
		if name != "RUSTUP_HOME" && name != "CARGO_HOME" {
			return nil, fmt.Errorf("project environment variable %s is not allowed", name)
		}
		if value == "" || !filepath.IsAbs(value) || strings.ContainsAny(value, "\x00\r\n") {
			return nil, fmt.Errorf("project environment variable %s is invalid", name)
		}
		result[name] = value
	}
	if (result["RUSTUP_HOME"] == "") != (result["CARGO_HOME"] == "") {
		return nil, errors.New("project environment Rust variables must appear together")
	}
	return result, nil
}

func mergeEnvironment(base []string, replacements map[string]string) []string {
	values := make(map[string]string, len(base)+len(replacements))
	for _, entry := range base {
		if name, value, found := strings.Cut(entry, "="); found {
			values[name] = value
		}
	}
	for name, value := range replacements {
		values[name] = value
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, name+"="+values[name])
	}
	return result
}

func environmentValue(environment []string, name string) string {
	prefix := name + "="
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}
