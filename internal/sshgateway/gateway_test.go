package sshgateway

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

	notGit := filepath.Join(fixture.root, "project", "not-git")
	mustMkdir(t, notGit)
	options.Worktree = notGit
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
	profile := filepath.Join(t.TempDir(), "profile.env")
	mustWrite(t, profile, strings.Join([]string{
		"export SODA_PROFILE=rust",
		"export RUSTUP_HOME=/opt/soda/rustup",
		"export CARGO_HOME=/opt/soda/cargo",
		"export PATH=/opt/soda/bin:/opt/soda/cargo/bin:$PATH",
		"",
	}, "\n"))
	mustWrite(t, filepath.Join(fixture.project, ".soda", "env"), "source "+profile+"\n")

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
			assertEnv(t, invocation.Env, "SODA_WORKTREE", fixture.worktree)
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

func TestProfilePathDoesNotAddCurrentDirectoryWhenPathIsUnset(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	profile := filepath.Join(t.TempDir(), "profile.env")
	mustWrite(t, profile, "export SODA_PROFILE=go\nexport PATH=/opt/go/bin:$PATH\n")
	mustWrite(t, filepath.Join(fixture.project, ".soda", "env"), "source "+profile+"\n")
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
		name    string
		link    string
		profile string
	}{
		{name: "missing source", link: "export PROFILE=x\n"},
		{name: "multiple sources", link: "source /one\nsource /two\n"},
		{name: "relative profile", link: "source relative/env\n"},
		{name: "unsupported line", profile: "export SODA_PROFILE=go\nrun something\nexport PATH=/opt/go:$PATH\n"},
		{name: "unsupported variable", profile: "export SODA_PROFILE=go\nexport SECRET=value\nexport PATH=/opt/go:$PATH\n"},
		{name: "malformed path", profile: "export SODA_PROFILE=go\nexport PATH=/opt/go\n"},
		{name: "missing profile", profile: "export PATH=/opt/go:$PATH\n"},
		{name: "duplicate variable", profile: "export SODA_PROFILE=go\nexport SODA_PROFILE=rust\nexport PATH=/opt/go:$PATH\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFixture(t)
			link := test.link
			if test.profile != "" {
				profile := filepath.Join(t.TempDir(), "profile.env")
				mustWrite(t, profile, test.profile)
				link = "source " + profile + "\n"
			}
			mustWrite(t, filepath.Join(fixture.project, ".soda", "env"), link)
			if _, err := BuildInvocation(fixture.options()); err == nil {
				t.Fatal("malformed environment unexpectedly succeeded")
			}
		})
	}
}

func TestNearestProjectEnvironmentWins(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	validProfile := filepath.Join(t.TempDir(), "valid.env")
	mustWrite(t, validProfile, "export SODA_PROFILE=go\nexport PATH=/opt/go:$PATH\n")
	mustWrite(t, filepath.Join(fixture.project, ".soda", "env"), "source "+validProfile+"\n")
	mustWrite(t, filepath.Join(fixture.worktree, ".soda", "env"), "not a source\n")
	if _, err := BuildInvocation(fixture.options()); err == nil {
		t.Fatal("invalid nearest environment was skipped")
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
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "projects")
	project := filepath.Join(root, "example")
	worktree := filepath.Join(project, "worktrees", "alice")
	mustMkdir(t, filepath.Join(worktree, ".git"))
	root = mustCanonical(t, root)
	project = mustCanonical(t, project)
	worktree = mustCanonical(t, worktree)
	return fixture{root: root, project: project, worktree: worktree}
}

func (fixture fixture) options() Options {
	return Options{
		Actor:        "alice-2",
		Worktree:     fixture.worktree,
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
