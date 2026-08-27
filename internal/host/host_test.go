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
	for _, binary := range []string{"git", "ssh-keygen"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Skipf("%s unavailable", binary)
		}
	}
	root := t.TempDir()
	system := New(root, false)
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
	key := domain.SSHDeviceKey{ID: uuid.NewString(), PersonID: person.ID, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA alice", Fingerprint: "SHA256:test", IdentityFileHint: "~/.ssh/id_ed25519"}
	if err = system.ReconcileAuthorizedKeys(context.Background(), project, []domain.ProjectAccess{{Person: person, Worktree: tree, Keys: []domain.SSHDeviceKey{key}}}); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"user.name": person.DisplayName, "user.email": person.Email, "core.bare": "false"} {
		output, err := exec.Command("git", "-C", tree.Path, "config", "--worktree", "--get", key).Output()
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(output)) != want {
			t.Fatalf("%s = %q", key, output)
		}
	}
	keys, err := os.ReadFile(filepath.Join(system.AuthorizedKeysRoot, project.UnixUser))
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "demo", ".soda", "people", "alice", "home")
	if !strings.Contains(string(keys), "--actor alice --project demo --worktree "+tree.Path+" --home "+home) {
		t.Fatalf("authorized_keys = %q", keys)
	}
	workspace, err := os.Readlink(filepath.Join(home, "workspace"))
	if err != nil || workspace != tree.Path {
		t.Fatalf("workspace link = %q, %v", workspace, err)
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
	system := New(root, false)
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.GitProjectSource{RemoteURL: remote}}
	cleanup, err := system.CreateProject(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup(context.Background()) })
	if err = system.EnsureRepository(ctx, project); err == nil {
		t.Fatal("missing private remote unexpectedly cloned")
	}
	if _, statErr := os.Stat(filepath.Join(root, project.Slug, "repository.git")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed clone was not removed: %v", statErr)
	}
	if _, err = system.Runner.Run(ctx, "git", []string{"init", "--bare", "--initial-branch=trunk", remote}, nil, ""); err != nil {
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
	if err = system.EnsureRepository(ctx, project); err != nil {
		t.Fatalf("clone retry failed: %v", err)
	}
	branch, err := system.DefaultBranch(ctx, project)
	if err != nil || branch != "trunk" {
		t.Fatalf("default branch = %q, %v", branch, err)
	}
	if err = system.EnsureRepository(ctx, project); err != nil {
		t.Fatalf("successful repository was not idempotent: %v", err)
	}
}

type recordedCall struct {
	name  string
	args  []string
	input string
}

type recordingRunner struct {
	calls    []recordedCall
	failName string
}

func (r *recordingRunner) Run(_ context.Context, name string, args []string, _ map[string]string, input string) (string, error) {
	r.calls = append(r.calls, recordedCall{name: name, args: append([]string(nil), args...), input: input})
	if name == r.failName {
		r.failName = ""
		return "", errors.New("injected " + name + " failure")
	}
	return "", nil
}

func TestCreatePersonUsesRelaxedSixCharacterPasswordPolicy(t *testing.T) {
	runner := &recordingRunner{}
	system := New(t.TempDir(), true)
	system.Runner = runner
	person := domain.Person{Username: "alice", Role: domain.RoleDeveloper}
	for _, password := range []string{"simple", "with spaces", "colon:allowed"} {
		cleanup, err := system.CreatePerson(context.Background(), person, password)
		if err != nil {
			t.Fatalf("password %q: %v", password, err)
		}
		if err = cleanup(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	before := len(runner.calls)
	for _, password := range []string{"short", "bad\npassword"} {
		if _, err := system.CreatePerson(context.Background(), person, password); err == nil {
			t.Fatalf("invalid password %q accepted", password)
		}
	}
	calls := runner.calls[before:]
	if hasCallNamed(calls, "useradd") || hasCallNamed(calls, "chpasswd") {
		t.Fatalf("invalid password changed host state: %#v", calls)
	}
}

func TestPersonAccountsDoNotUseSodaRoleGroups(t *testing.T) {
	runner := &recordingRunner{}
	system := New(t.TempDir(), true)
	system.Runner = runner
	person := domain.Person{Username: "alice", Role: domain.RoleAdmin}
	if _, err := system.CreatePerson(context.Background(), person, "simple"); err != nil {
		t.Fatal(err)
	}
	if !hasCall(runner.calls, "useradd", "--create-home", "--shell", "/sbin/nologin", "alice") {
		t.Fatalf("person useradd call = %#v", runner.calls)
	}
	for _, name := range []string{"groupadd", "usermod", "gpasswd"} {
		if hasCallNamed(runner.calls, name) {
			t.Fatalf("unexpected role-group command %q: %#v", name, runner.calls)
		}
	}
}

func TestImportPersonOnlyVerifiesLinuxAccount(t *testing.T) {
	runner := &recordingRunner{}
	system := New(t.TempDir(), true)
	system.Runner = runner
	if _, err := system.ImportPerson(context.Background(), domain.Person{Username: "alice", Role: domain.RoleDeveloper}); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || !hasCall(runner.calls, "getent", "passwd", "alice") {
		t.Fatalf("import calls = %#v", runner.calls)
	}
}

func TestProjectAuthorizedKeysRemainRootOwnedOutsideProjectHome(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{}
	system := New(root, true)
	system.Runner = runner
	system.AuthorizedKeysRoot = filepath.Join(t.TempDir(), "authorized_keys")
	project := domain.Project{Slug: "demo", UnixUser: "soda-p-demo", Source: domain.GitProjectSource{RemoteURL: "https://example.test/demo.git"}}
	if err := os.MkdirAll(filepath.Join(root, project.Slug), 0o755); err != nil {
		t.Fatal(err)
	}
	cleanup, err := system.CreateProject(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup(context.Background()) })
	keyFile := filepath.Join(system.AuthorizedKeysRoot, project.UnixUser)
	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("authorized key mode = %o", info.Mode().Perm())
	}
	if _, err = os.Stat(filepath.Join(root, project.Slug, ".ssh", "authorized_keys")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("project-owned authorized_keys exists: %v", err)
	}
	if !hasCall(runner.calls, "chown", "root:root", system.AuthorizedKeysRoot, keyFile) {
		t.Fatalf("missing root ownership call: %#v", runner.calls)
	}
	if hasCall(runner.calls, "chown", "--recursive", project.UnixUser+":"+project.UnixUser, keyFile) {
		t.Fatalf("project owns authorized_keys: %#v", runner.calls)
	}
	person := domain.Person{ID: uuid.NewString(), Username: "alice"}
	tree := domain.Worktree{PersonID: person.ID, Path: "/srv/soda/projects/demo/worktrees/alice"}
	keys := []domain.SSHDeviceKey{
		{ID: uuid.NewString(), PersonID: person.ID, Label: "Workstation", PublicKey: "ssh-ed25519 AQ== workstation", Fingerprint: "SHA256:z"},
		{ID: uuid.NewString(), PersonID: person.ID, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA laptop", Fingerprint: "SHA256:a"},
	}
	if err = system.ReconcileAuthorizedKeys(context.Background(), project, []domain.ProjectAccess{{Person: person, Worktree: tree, Keys: keys}}); err != nil {
		t.Fatal(err)
	}
	if !hasCall(runner.calls, "chown", "root:root", keyFile) {
		t.Fatalf("authorized key rewrite was not restored to root ownership: %#v", runner.calls)
	}
	contents, err := os.ReadFile(keyFile)
	wantFirst := "command=\"/usr/libexec/soda/soda-ssh --actor alice --project demo --worktree /srv/soda/projects/demo/worktrees/alice --home " + filepath.Join(root, "demo", ".soda", "people", "alice", "home") + "\" ssh-ed25519 AAAA"
	if err != nil || !strings.HasPrefix(string(contents), wantFirst) || strings.Contains(string(contents), " laptop") {
		t.Fatalf("authorized key contents = %q, %v", contents, err)
	}
}

func TestCreatePersonCleansPartialUserAfterChpasswdFailure(t *testing.T) {
	runner := &recordingRunner{failName: "chpasswd"}
	system := New(t.TempDir(), true)
	system.Runner = runner
	person := domain.Person{Username: "alice", Role: domain.RoleDeveloper}
	if _, err := system.CreatePerson(context.Background(), person, "simple"); err == nil {
		t.Fatal("expected chpasswd failure")
	}
	if !hasCall(runner.calls, "userdel", "--remove", person.Username) {
		t.Fatalf("partial user was not cleaned: %#v", runner.calls)
	}
}

func TestCreateProjectCleansPartialAccountAndFiles(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{failName: "ssh-keygen"}
	system := New(root, true)
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
	system := New(root, false)
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
		if call.name == name && strings.Join(call.args, "\x00") == strings.Join(args, "\x00") {
			return true
		}
	}
	return false
}

func hasCallNamed(calls []recordedCall, name string) bool {
	for _, call := range calls {
		if call.name == name {
			return true
		}
	}
	return false
}

func TestPublicKeyValidation(t *testing.T) {
	if err := ValidatePublicKey("", true); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "ssh-ed25519 key\ncommand", "ecdsa-sha2-nistp256 key"} {
		if err := ValidatePublicKey(key, false); err == nil {
			t.Fatalf("accepted %q", key)
		}
	}
}
