package sshgateway

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
)

type captureExecutor struct {
	invocation Invocation
	err        error
}

func (executor *captureExecutor) Exec(invocation Invocation) error {
	executor.invocation = invocation
	return executor.err
}

func TestActorValidation(t *testing.T) {
	t.Parallel()
	for _, actor := range []string{"alice", "alice-2", "2-alice", "-alice"} {
		if err := validateActor(actor); err != nil {
			t.Errorf("validateActor(%q) returned %v", actor, err)
		}
	}
	for _, actor := range []string{"", "Alice", "alice_b", "alice/b", "alice\nbob", "álîce"} {
		if err := validateActor(actor); err == nil {
			t.Errorf("validateActor(%q) unexpectedly succeeded", actor)
		}
	}
}

func TestWorktreeContainment(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)

	if _, err := BuildInvocation(fixture.options()); err != nil {
		t.Fatalf("valid worktree was rejected: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside")
	mustMkdir(t, filepath.Join(outside, ".git"))
	options := fixture.options()
	options.Worktree = outside
	if _, err := BuildInvocation(options); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("outside worktree error = %v", err)
	}

	if err := os.RemoveAll(filepath.Join(fixture.worktree, ".git")); err != nil {
		t.Fatal(err)
	}
	options = fixture.options()
	if _, err := BuildInvocation(options); err == nil || !strings.Contains(err.Error(), "not a Git") {
		t.Fatalf("non-Git worktree error = %v", err)
	}
}

func TestCanonicalContainmentResolvesSymlinks(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)

	insideLink := filepath.Join(fixture.root, "inside-link")
	if err := os.Symlink(fixture.worktree, insideLink); err != nil {
		t.Fatal(err)
	}
	options := fixture.options()
	options.Worktree = insideLink
	invocation, err := BuildInvocation(options)
	if err != nil {
		t.Fatalf("inside symlink was rejected: %v", err)
	}
	if invocation.Dir != fixture.worktree {
		t.Fatalf("canonical worktree = %q, want %q", invocation.Dir, fixture.worktree)
	}

	outside := filepath.Join(t.TempDir(), "outside")
	mustMkdir(t, filepath.Join(outside, ".git"))
	escapeLink := filepath.Join(fixture.root, "escape-link")
	if err := os.Symlink(outside, escapeLink); err != nil {
		t.Fatal(err)
	}
	options.Worktree = escapeLink
	if _, err := BuildInvocation(options); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("escaping symlink error = %v", err)
	}

	rootLink := filepath.Join(t.TempDir(), "projects-link")
	if err := os.Symlink(fixture.root, rootLink); err != nil {
		t.Fatal(err)
	}
	options = fixture.options()
	options.ProjectsRoot = rootLink
	if _, err := BuildInvocation(options); err != nil {
		t.Fatalf("canonical root symlink was rejected: %v", err)
	}
}

func TestSessionCommandsAndFinalEnvironment(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	mustWriteProjectEnvironment(t, fixture.project, domain.ProjectEnvironment{
		Profile: "rust", Path: []string{"/opt/soda/bin", "/opt/soda/cargo/bin"},
		Variables: map[string]string{"RUSTUP_HOME": "/opt/soda/rustup", "CARGO_HOME": "/opt/soda/cargo"},
	})

	tests := []struct {
		name     string
		original string
		shell    string
		path     string
		argv     []string
		wantPWD  bool
	}{
		{name: "login shell", shell: "/bin/bash", path: "/bin/bash", argv: []string{"/bin/bash", "-l"}},
		{name: "command", original: "printf hello", path: "/bin/bash", argv: []string{"/bin/bash", "-lc", "printf hello"}, wantPWD: true},
		{name: "SFTP", original: "internal-sftp", path: sftpServer, argv: []string{sftpServer}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := fixture.options()
			options.OriginalCommand = test.original
			options.Shell = test.shell
			options.Environment = []string{"PATH=/usr/local/bin:/usr/bin", "LANG=en_US.UTF-8", "SODA_ACTOR=wrong"}
			invocation, err := BuildInvocation(options)
			if err != nil {
				t.Fatal(err)
			}
			if invocation.Path != test.path || !reflect.DeepEqual(invocation.Argv, test.argv) {
				t.Fatalf("command = %q %#v, want %q %#v", invocation.Path, invocation.Argv, test.path, test.argv)
			}
			if invocation.Dir != fixture.worktree {
				t.Fatalf("Dir = %q", invocation.Dir)
			}
			assertEnv(t, invocation.Env, "SODA_ACTOR", "alice-2")
			assertEnv(t, invocation.Env, "SODA_PROJECT", "example")
			assertEnv(t, invocation.Env, "SODA_WORKTREE", fixture.worktree)
			assertEnv(t, invocation.Env, "HOME", fixture.home)
			assertEnv(t, invocation.Env, "XDG_CONFIG_HOME", filepath.Join(fixture.home, ".config"))
			assertEnv(t, invocation.Env, "XDG_CACHE_HOME", filepath.Join(fixture.home, ".cache"))
			assertEnv(t, invocation.Env, "XDG_DATA_HOME", filepath.Join(fixture.home, ".local", "share"))
			assertEnv(t, invocation.Env, "XDG_STATE_HOME", filepath.Join(fixture.home, ".local", "state"))
			assertEnv(t, invocation.Env, "SODA_PROFILE", "rust")
			assertEnv(t, invocation.Env, "RUSTUP_HOME", "/opt/soda/rustup")
			assertEnv(t, invocation.Env, "CARGO_HOME", "/opt/soda/cargo")
			assertEnv(t, invocation.Env, "PATH", "/opt/soda/bin:/opt/soda/cargo/bin:/usr/local/bin:/usr/bin")
			assertEnv(t, invocation.Env, "LANG", "en_US.UTF-8")
			if got := environmentValue(invocation.Env, "PWD"); test.wantPWD && got != fixture.worktree {
				t.Fatalf("PWD = %q, want %q", got, fixture.worktree)
			}
		})
	}
}

func TestDefaultLoginShell(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	options := fixture.options()
	invocation, err := BuildInvocation(options)
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Path != "/bin/bash" || !reflect.DeepEqual(invocation.Argv, []string{"/bin/bash", "-l"}) {
		t.Fatalf("default command = %q %#v", invocation.Path, invocation.Argv)
	}
}

func TestInteractiveBannerAndSilentNoninteractiveSessions(t *testing.T) {
	fixture := newFixture(t)
	options := fixture.options()
	options.Environment = append(options.Environment, "SSH_TTY=/dev/pts/1")
	invocation, err := BuildInvocation(options)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Soda OS", "Person: alice-2", "Project: example", "Branch: people/alice-2", "Workspace: " + fixture.worktree} {
		if !strings.Contains(invocation.Banner, text) {
			t.Fatalf("banner %q does not contain %q", invocation.Banner, text)
		}
	}
	for _, command := range []string{"printf hello", "internal-sftp"} {
		options.OriginalCommand = command
		invocation, err = BuildInvocation(options)
		if err != nil {
			t.Fatal(err)
		}
		if invocation.Banner != "" {
			t.Fatalf("%q session received banner %q", command, invocation.Banner)
		}
	}
}

func TestProfilePathDoesNotAddCurrentDirectoryWhenPathIsUnset(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	mustWriteProjectEnvironment(t, fixture.project, domain.ProjectEnvironment{Profile: "go", Path: []string{"/opt/go/bin"}})
	options := fixture.options()
	options.Environment = []string{"LANG=C"}
	invocation, err := BuildInvocation(options)
	if err != nil {
		t.Fatal(err)
	}
	assertEnv(t, invocation.Env, "PATH", "/opt/go/bin")
}

func TestStrictProjectEnvironmentParsing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		env  domain.ProjectEnvironment
	}{
		{name: "invalid JSON", raw: "{"},
		{name: "empty PATH", env: domain.ProjectEnvironment{Profile: "go"}},
		{name: "relative PATH", env: domain.ProjectEnvironment{Profile: "go", Path: []string{"bin"}}},
		{name: "unsupported variable", env: domain.ProjectEnvironment{Profile: "go", Path: []string{"/opt/go"}, Variables: map[string]string{"SECRET": "value"}}},
		{name: "unpaired Rust variable", env: domain.ProjectEnvironment{Profile: "rust", Path: []string{"/opt/rust"}, Variables: map[string]string{"RUSTUP_HOME": "/opt/rustup"}}},
		{name: "invalid profile", env: domain.ProjectEnvironment{Profile: "bad\nprofile", Path: []string{"/opt/go"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFixture(t)
			if test.raw != "" {
				mustWrite(t, filepath.Join(fixture.project, ".soda", "environment.json"), test.raw)
			} else {
				mustWriteProjectEnvironment(t, fixture.project, test.env)
			}
			if _, err := BuildInvocation(fixture.options()); err == nil {
				t.Fatal("malformed environment unexpectedly succeeded")
			}
		})
	}
}

func TestProjectRootEnvironmentIsAuthoritative(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	mustWriteProjectEnvironment(t, fixture.project, domain.ProjectEnvironment{Profile: "go", Path: []string{"/opt/go"}})
	mustWrite(t, filepath.Join(fixture.worktree, ".soda", "environment.json"), "{")
	if _, err := BuildInvocation(fixture.options()); err != nil {
		t.Fatalf("project environment was not authoritative: %v", err)
	}
}

func TestRunUsesInjectedExecutor(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	executor := &captureExecutor{}
	if err := Run(fixture.options(), executor); err != nil {
		t.Fatal(err)
	}
	if executor.invocation.Dir != fixture.worktree {
		t.Fatalf("executor received Dir %q", executor.invocation.Dir)
	}

	executor.err = errors.New("exec denied")
	if err := Run(fixture.options(), executor); err == nil || !strings.Contains(err.Error(), "exec denied") {
		t.Fatalf("executor error = %v", err)
	}
}

type fixture struct {
	root     string
	project  string
	worktree string
	home     string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "projects")
	project := filepath.Join(root, "example")
	worktree := filepath.Join(project, "worktrees", "alice-2")
	home := filepath.Join(project, ".soda", "people", "alice-2", "home")
	mustMkdir(t, filepath.Join(worktree, ".git"))
	mustMkdir(t, home)
	mustWriteProjectEnvironment(t, project, domain.ProjectEnvironment{Profile: "go", Path: []string{"/opt/go/bin"}})
	root = mustCanonical(t, root)
	project = mustCanonical(t, project)
	worktree = mustCanonical(t, worktree)
	home = mustCanonical(t, home)
	return fixture{root: root, project: project, worktree: worktree, home: home}
}

func (fixture fixture) options() Options {
	return Options{
		Actor:        "alice-2",
		Project:      "example",
		Worktree:     fixture.worktree,
		Home:         fixture.home,
		ProjectsRoot: fixture.root,
		Environment:  []string{"PATH=/usr/bin"},
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWriteProjectEnvironment(t *testing.T, project string, environment domain.ProjectEnvironment) {
	t.Helper()
	contents, err := json.Marshal(environment)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(project, ".soda", "environment.json"), string(contents))
}

func mustCanonical(t *testing.T, path string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func assertEnv(t *testing.T, environment []string, name, want string) {
	t.Helper()
	if got := environmentValue(environment, name); got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}
