// Package sshgateway validates and enters a Soda project worktree for an SSH
// session forced by the project's authorized_keys entry.
package sshgateway

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	defaultProjectsRoot = "/srv/soda/projects"
	sftpServer          = "/usr/libexec/openssh/sftp-server"
)

// Options contains the trusted forced-command arguments and inherited SSH
// session environment used to build a gateway invocation.
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

// Invocation is the fully validated process replacement requested by a Soda
// SSH session.
type Invocation struct {
	Path   string
	Argv   []string
	Env    []string
	Dir    string
	Banner string
}

// Executor replaces the gateway process with the requested session process.
// It is an interface so validation and the final argv/environment can be
// tested without replacing the test process.
type Executor interface {
	Exec(Invocation) error
}

// UnixExecutor is the production process-replacing executor.
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

// Run validates a session and delegates process replacement to executor.
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

// BuildInvocation validates all inputs without executing a process.
func BuildInvocation(options Options) (Invocation, error) {
	if err := validateActor(options.Actor); err != nil {
		return Invocation{}, err
	}
	if err := validateActor(options.Project); err != nil {
		return Invocation{}, errors.New("invalid Soda project")
	}

	projectsRoot := options.ProjectsRoot
	if projectsRoot == "" {
		projectsRoot = defaultProjectsRoot
	}
	root, err := canonicalDirectory(projectsRoot, "projects root")
	if err != nil {
		return Invocation{}, err
	}
	worktree, err := canonicalDirectory(options.Worktree, "worktree")
	if err != nil {
		return Invocation{}, err
	}
	if !containedBy(root, worktree) {
		return Invocation{}, errors.New("worktree is outside the Soda projects root")
	}
	projectRoot, err := canonicalDirectory(filepath.Join(root, options.Project), "project root")
	if err != nil {
		return Invocation{}, err
	}
	expectedWorktree := filepath.Join(projectRoot, "worktrees", options.Actor)
	if worktree != expectedWorktree {
		return Invocation{}, errors.New("worktree does not match the Soda actor and project")
	}
	home, err := canonicalDirectory(options.Home, "session home")
	if err != nil {
		return Invocation{}, err
	}
	expectedHome := filepath.Join(projectRoot, ".soda", "people", options.Actor, "home")
	if home != expectedHome {
		return Invocation{}, errors.New("session home does not match the Soda actor and project")
	}
	if _, err := os.Stat(filepath.Join(worktree, ".git")); err != nil {
		if os.IsNotExist(err) {
			return Invocation{}, errors.New("worktree is not a Git worktree")
		}
		return Invocation{}, fmt.Errorf("inspect worktree Git metadata: %w", err)
	}

	path, argv, err := sessionCommand(options.OriginalCommand, options.Shell)
	if err != nil {
		return Invocation{}, err
	}
	environment := mergeEnvironment(options.Environment, map[string]string{
		"HOME":            home,
		"PWD":             worktree,
		"SODA_ACTOR":      options.Actor,
		"SODA_PROJECT":    options.Project,
		"SODA_WORKTREE":   worktree,
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
	})

	profile, err := projectEnvironment(worktree, root)
	if err != nil {
		return Invocation{}, err
	}
	if profile != nil {
		inheritedPath := environmentValue(environment, "PATH")
		if inheritedPath != "" {
			profile["PATH"] = profile["PATH"] + ":" + inheritedPath
		}
		environment = mergeEnvironment(environment, profile)
	}

	banner := ""
	if options.OriginalCommand == "" && environmentValue(options.Environment, "SSH_TTY") != "" {
		banner = fmt.Sprintf("Soda OS\nPerson: %s\nProject: %s\nBranch: people/%s\nWorkspace: %s\n\n", options.Actor, options.Project, options.Actor, worktree)
	}
	return Invocation{Path: path, Argv: argv, Env: environment, Dir: worktree, Banner: banner}, nil
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

func projectEnvironment(worktree, projectsRoot string) (map[string]string, error) {
	for directory := worktree; containedBy(projectsRoot, directory); directory = filepath.Dir(directory) {
		environmentLink := filepath.Join(directory, ".soda", "env")
		info, err := os.Stat(environmentLink)
		if err == nil {
			if !info.Mode().IsRegular() {
				return nil, errors.New("project environment link is not a regular file")
			}
			return loadProjectEnvironment(environmentLink)
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect project environment: %w", err)
		}
		if directory == projectsRoot {
			break
		}
	}
	return nil, nil
}

func loadProjectEnvironment(linkPath string) (map[string]string, error) {
	contents, err := os.ReadFile(linkPath)
	if err != nil {
		return nil, fmt.Errorf("read project environment: %w", err)
	}
	lines := nonemptyLines(string(contents))
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "source ") {
		return nil, errors.New("project environment does not name exactly one profile")
	}
	profilePath := strings.TrimSpace(strings.TrimPrefix(lines[0], "source "))
	if profilePath == "" || !filepath.IsAbs(profilePath) || strings.ContainsAny(profilePath, "\x00\r\n") {
		return nil, errors.New("project environment profile path is invalid")
	}
	profileContents, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("read profile environment: %w", err)
	}

	environment := make(map[string]string)
	for _, line := range nonemptyLines(string(profileContents)) {
		if !strings.HasPrefix(line, "export ") {
			return nil, fmt.Errorf("profile environment contains unsupported line %q", line)
		}
		name, value, found := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		if !found || value == "" {
			return nil, fmt.Errorf("profile environment contains malformed assignment %q", line)
		}
		if _, duplicate := environment[name]; duplicate {
			return nil, fmt.Errorf("profile environment defines %s more than once", name)
		}
		switch name {
		case "SODA_PROFILE", "RUSTUP_HOME", "CARGO_HOME":
			environment[name] = value
		case "PATH":
			prefix, ok := strings.CutSuffix(value, ":$PATH")
			if !ok || prefix == "" {
				return nil, errors.New("profile PATH has an unsupported form")
			}
			environment[name] = prefix
		default:
			return nil, fmt.Errorf("profile environment variable %s is not allowed", name)
		}
	}
	if environment["SODA_PROFILE"] == "" || environment["PATH"] == "" {
		return nil, errors.New("profile environment is incomplete")
	}
	return environment, nil
}

func nonemptyLines(contents string) []string {
	var lines []string
	for _, line := range strings.Split(contents, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func mergeEnvironment(base []string, replacements map[string]string) []string {
	result := make([]string, 0, len(base)+len(replacements))
	seen := make(map[string]bool, len(replacements))
	for _, entry := range base {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if value, replace := replacements[name]; replace {
			if !seen[name] {
				result = append(result, name+"="+value)
				seen[name] = true
			}
			continue
		}
		result = append(result, entry)
	}
	for name, value := range replacements {
		if !seen[name] {
			result = append(result, name+"="+value)
		}
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
