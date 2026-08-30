package host

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
	"github.com/google/uuid"
)

func TestCreatesEmptyProjectAndAttributedWorktree(t *testing.T) {
	root := t.TempDir()
	for _, binary := range []string{"git", "ssh-keygen"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Skipf("%s unavailable", binary)
		}
	}
	system := New(root)
	system.Runner = managedTestRunner{}
	system.AuthorizedKeysRoot = filepath.Join(t.TempDir(), "authorized_keys")
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	projectCleanup, err := system.CreateProject(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectCleanup(context.Background()) })
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice Example", Email: "alice@example.test", Role: domain.RoleDeveloper}
	tree := domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: person.ID, Name: "default", Branch: "people/alice", Path: filepath.Join(root, "demo", "worktrees", "alice")}
	worktreeCleanup, err := system.CreateWorktree(context.Background(), project, person, tree, "main")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worktreeCleanup(context.Background()) })
	assertWorktreeIdentity(t, tree, person)
	info, err := os.Stat(tree.Path)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("personal worktree mode = %v, %v", info, err)
	}
	workspace, err := os.Readlink(filepath.Join(root, "demo", ".soda", "people", "alice", "home", "workspace"))
	if err != nil || workspace != tree.Path {
		t.Fatalf("workspace link = %q, %v", workspace, err)
	}
}

func assertWorktreeIdentity(t *testing.T, tree domain.Worktree, person domain.Person) {
	t.Helper()
	for key, want := range map[string]string{
		"user.name": person.DisplayName, "user.email": person.Email, "core.bare": "false",
		"core.sshCommand": "ssh -i /home/alice/.ssh/soda_git_ed25519 -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new",
	} {
		output, err := exec.Command("git", "-C", tree.Path, "config", "--worktree", "--get", key).Output()
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(output)) != want {
			t.Fatalf("%s = %q", key, output)
		}
	}
}

func TestGitCloneRetryResolvesNonMainDefaultBranch(t *testing.T) {
	for _, binary := range []string{"git", "ssh-keygen"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Skipf("%s unavailable", binary)
		}
	}
	ctx := context.Background()
	root := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	system := New(root)
	system.Runner = managedTestRunner{}
	system.AuthorizedKeysRoot = filepath.Join(t.TempDir(), "authorized_keys")
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.GitProjectSource{RemoteURL: remote}}
	bootstrap := domain.Person{ID: uuid.NewString(), Username: "alice"}
	cleanup, err := system.CreateProject(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup(context.Background()) })
	if err = system.EnsureRepository(ctx, project, bootstrap); err == nil {
		t.Fatal("missing private remote unexpectedly cloned")
	}
	if _, err := os.Stat(filepath.Join(root, project.Slug, "repository.git")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed clone was not removed: %v", err)
	}
	initializeTestRemote(t, ctx, system, remote)
	if err = system.EnsureRepository(ctx, project, bootstrap); err != nil {
		t.Fatalf("clone retry failed: %v", err)
	}
	branch, err := system.DefaultBranch(ctx, project)
	if err != nil || branch != "trunk" {
		t.Fatalf("default branch = %q, %v", branch, err)
	}
	if err = system.EnsureRepository(ctx, project, bootstrap); err != nil {
		t.Fatalf("successful repository was not idempotent: %v", err)
	}
}

func initializeTestRemote(t *testing.T, ctx context.Context, system *System, remote string) {
	t.Helper()
	if _, err := system.Runner.Run(ctx, "git", []string{"init", "--bare", "--initial-branch=trunk", remote}, nil, ""); err != nil {
		t.Fatal(err)
	}
	tree, err := system.Runner.Run(ctx, "git", []string{"--git-dir", remote, "mktree"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	identity := map[string]string{"GIT_AUTHOR_NAME": "Soda Test", "GIT_AUTHOR_EMAIL": "soda@example.test", "GIT_COMMITTER_NAME": "Soda Test", "GIT_COMMITTER_EMAIL": "soda@example.test"}
	commit, err := system.Runner.Run(ctx, "git", []string{"--git-dir", remote, "commit-tree", strings.TrimSpace(tree), "-m", "Initialize test remote"}, identity, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = system.Runner.Run(ctx, "git", []string{"--git-dir", remote, "update-ref", "refs/heads/trunk", strings.TrimSpace(commit)}, nil, ""); err != nil {
		t.Fatal(err)
	}
}

type recordedCall struct {
	name        string
	args        []string
	environment map[string]string
	input       string
}

type managedTestRunner struct{ ExecRunner }

func (managedTestRunner) Run(ctx context.Context, name string, args []string, environment map[string]string, input string) (string, error) {
	switch name {
	case "useradd":
		for index, arg := range args {
			if arg == "--home-dir" && index+1 < len(args) {
				return "", os.MkdirAll(args[index+1], 0o755)
			}
		}
		return "", nil
	case "userdel", "usermod", "gpasswd", "chown", "restorecon":
		return "", nil
	default:
		return ExecRunner{}.Run(ctx, name, args, environment, input)
	}
}

type recordingRunner struct {
	calls    []recordedCall
	failName string
}

func (r *recordingRunner) Run(ctx context.Context, name string, args []string, environment map[string]string, input string) (string, error) {
	r.calls = append(r.calls, recordedCall{name: name, args: append([]string(nil), args...), environment: environment, input: input})
	if name == r.failName {
		r.failName = ""
		return "", errors.New("injected " + name + " failure")
	}
	if name == "ssh-keygen" {
		return ExecRunner{}.Run(ctx, name, args, environment, input)
	}
	return "", nil
}

func TestCreatePersonUsesRelaxedSixCharacterPasswordPolicy(t *testing.T) {
	runner := &recordingRunner{}
	system := New(t.TempDir())
	system.PeopleRoot = t.TempDir()
	system.Runner = runner
	person := domain.Person{Username: "alice", Role: domain.RoleDeveloper}
	for _, password := range []string{"simple", "with spaces", "colon:allowed"} {
		_, cleanup, err := system.CreatePerson(context.Background(), person, password)
		if err != nil {
			t.Fatalf("password %q: %v", password, err)
		}
		if err = cleanup(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	before := len(runner.calls)
	for _, password := range []string{"short", "bad\npassword"} {
		if _, _, err := system.CreatePerson(context.Background(), person, password); err == nil {
			t.Fatalf("invalid password %q accepted", password)
		}
	}
	calls := runner.calls[before:]
	if hasCall(calls, "useradd") || hasCall(calls, "chpasswd") {
		t.Fatalf("invalid password changed host state: %#v", calls)
	}
}

func TestPersonAccountsUsePersonalShellAndSodaPeopleGroup(t *testing.T) {
	runner := &recordingRunner{}
	system := New(t.TempDir())
	system.PeopleRoot = t.TempDir()
	system.Runner = runner
	person := domain.Person{Username: "alice", Role: domain.RoleAdmin}
	if _, _, err := system.CreatePerson(context.Background(), person, "simple"); err != nil {
		t.Fatal(err)
	}
	if !hasCall(runner.calls, "useradd", "--create-home", "--shell", "/bin/bash", "alice") {
		t.Fatalf("person useradd call = %#v", runner.calls)
	}
	if !hasCall(runner.calls, "usermod", "--append", "--groups", "soda-people", "alice") {
		t.Fatalf("person was not added to soda-people: %#v", runner.calls)
	}
}

func TestPersonAuthorizedKeysRemainRootOwnedOutsidePersonalHome(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{}
	system := New(root)
	system.Runner = runner
	system.AuthorizedKeysRoot = filepath.Join(t.TempDir(), "authorized_keys")
	person := domain.Person{ID: uuid.NewString(), Username: "alice"}
	keys := []domain.SSHDeviceKey{
		{ID: uuid.NewString(), PersonID: person.ID, Label: "Workstation", PublicKey: "ssh-ed25519 AQ==", Fingerprint: "SHA256:z"},
		{ID: uuid.NewString(), PersonID: person.ID, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA", Fingerprint: "SHA256:a"},
	}
	if err := system.ReconcileAuthorizedKeys(context.Background(), person, keys); err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(system.AuthorizedKeysRoot, person.Username)
	info, err := os.Stat(keyFile)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("authorized key mode = %v, %v", info, err)
	}
	if !hasCall(runner.calls, "chown", "root:root", keyFile) {
		t.Fatalf("authorized key rewrite was not restored to root ownership: %#v", runner.calls)
	}
	contents, err := os.ReadFile(keyFile)
	if err != nil || string(contents) != "ssh-ed25519 AAAA\nssh-ed25519 AQ==\n" || strings.Contains(string(contents), "command=") {
		t.Fatalf("authorized key contents = %q, %v", contents, err)
	}
}

func TestCreateSessionHomeIsPrivateToPerson(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{}
	system := New(root)
	system.Runner = runner
	project := domain.Project{Slug: "demo", UnixUser: "soda-p-demo"}
	person := domain.Person{Username: "alice"}
	tree := domain.Worktree{Path: filepath.Join(root, "demo", "worktrees", "alice")}
	if err := os.MkdirAll(tree.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	cleanup, err := system.createSessionHome(context.Background(), project, person, tree)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup(context.Background()) })
	personRoot := filepath.Join(root, "demo", ".soda", "people", "alice")
	info, statErr := os.Stat(personRoot)
	if statErr != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("person root mode = %v, %v", info, statErr)
	}
	if !hasCall(runner.calls, "chown", "--recursive", "alice:"+project.UnixUser, personRoot) {
		t.Fatalf("person root ownership call = %#v", runner.calls)
	}
}

func TestCreatePersonCleansPartialUserAfterChpasswdFailure(t *testing.T) {
	runner := &recordingRunner{failName: "chpasswd"}
	system := New(t.TempDir())
	system.Runner = runner
	person := domain.Person{Username: "alice", Role: domain.RoleDeveloper}
	if _, _, err := system.CreatePerson(context.Background(), person, "simple"); err == nil {
		t.Fatal("expected chpasswd failure")
	}
	if !hasCall(runner.calls, "userdel", "--remove", person.Username) {
		t.Fatalf("partial user was not cleaned: %#v", runner.calls)
	}
}

func TestCreateProjectCleansPartialAccountAndFiles(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{failName: "ssh-keygen"}
	system := New(root)
	system.Runner = runner
	system.AuthorizedKeysRoot = filepath.Join(t.TempDir(), "authorized_keys")
	project := domain.Project{Slug: "demo", UnixUser: "soda-p-demo", Source: domain.EmptyProjectSource{}}
	if _, err := system.CreateProject(context.Background(), project); err == nil {
		t.Fatal("expected ssh-keygen failure")
	}
	if !hasCall(runner.calls, "userdel", "--remove", project.UnixUser) {
		t.Fatalf("partial project account was not cleaned: %#v", runner.calls)
	}
	if _, err := os.Stat(filepath.Join(root, project.Slug)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial project home remains: %v", err)
	}
}

type failingGitRunner struct {
	ExecRunner
	call   int
	failAt int
}

func (r *failingGitRunner) Run(ctx context.Context, name string, args []string, environment map[string]string, input string) (string, error) {
	if name == "git" {
		r.call++
		if r.call == r.failAt {
			return "", errors.New("injected git failure")
		}
	}
	return r.ExecRunner.Run(ctx, name, args, environment, input)
}

func TestCreateWorktreeCleansPartialGitWorktreeAndBranch(t *testing.T) {
	for _, binary := range []string{"git", "ssh-keygen"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Skipf("%s unavailable", binary)
		}
	}
	root := t.TempDir()
	system := New(root)
	system.Runner = managedTestRunner{}
	system.AuthorizedKeysRoot = filepath.Join(t.TempDir(), "authorized_keys")
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", UnixUser: "soda-p-demo", Source: domain.EmptyProjectSource{}}
	projectCleanup, err := system.CreateProject(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectCleanup(context.Background()) })
	runner := &failingGitRunner{failAt: 2}
	system.Runner = runner
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper}
	tree := domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: person.ID, Name: "default", Branch: "people/alice", Path: filepath.Join(root, "demo", "worktrees", "alice")}
	if _, err = system.CreateWorktree(context.Background(), project, person, tree, "main"); err == nil {
		t.Fatal("expected worktree configuration failure")
	}
	if _, err = os.Stat(tree.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial worktree remains: %v", err)
	}
	if err = exec.Command("git", "--git-dir", filepath.Join(root, "demo", "repository.git"), "show-ref", "--verify", "refs/heads/"+tree.Branch).Run(); err == nil {
		t.Fatal("partial worktree branch remains")
	}
}

func hasCall(calls []recordedCall, name string, args ...string) bool {
	for _, call := range calls {
		if call.name == name && (len(args) == 0 || strings.Join(call.args, "\x00") == strings.Join(args, "\x00")) {
			return true
		}
	}
	return false
}
