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

func TestInstallerAdministratorDiscovery(t *testing.T) {
	account := installerAdministrator("root:x:0:0:root:/root:/bin/bash\nvincent:x:1000:1000:Vincent Example:/home/vincent:/bin/bash\n", "wheel:x:10:vincent\n")
	if account == nil || account.username != "vincent" || account.displayName != "Vincent Example" {
		t.Fatalf("account = %#v", account)
	}
}

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
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice Example", Email: "alice@example.test", Role: domain.RoleDeveloper, SSHPublicKey: "ssh-ed25519 AAAA alice"}
	tree := domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: person.ID, Name: "default", Branch: "people/alice", Path: filepath.Join(root, "demo", "worktrees", "alice")}
	worktreeCleanup, err := system.CreateWorktree(context.Background(), project, person, tree, "main")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worktreeCleanup(context.Background()) })
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
	if !strings.Contains(string(keys), "--actor alice --worktree "+tree.Path) {
		t.Fatalf("authorized_keys = %q", keys)
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

func TestCreatePersonRejectsOnlyPasswordDelimitersAndCleansAccount(t *testing.T) {
	runner := &recordingRunner{}
	system := New(t.TempDir(), true)
	system.Runner = runner
	person := domain.Person{Username: "alice", Role: domain.RoleDeveloper, SSHPublicKey: "ssh-ed25519 AAAA alice"}
	for _, password := range []string{"short", "with spaces", "colon:allowed"} {
		cleanup, err := system.CreatePerson(context.Background(), person, password)
		if err != nil {
			t.Fatalf("password %q: %v", password, err)
		}
		if err = cleanup(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	before := len(runner.calls)
	if _, err := system.CreatePerson(context.Background(), person, "bad\npassword"); err == nil {
		t.Fatal("line-delimited password accepted")
	}
	calls := runner.calls[before:]
	if !hasCall(calls, "userdel", "--remove", "alice") || hasCallNamed(calls, "chpasswd") {
		t.Fatalf("delimiter cleanup calls = %#v", calls)
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
	line := "command=\"/usr/libexec/soda/soda-ssh --actor alice --worktree /srv/soda/projects/demo/worktrees/alice\" ssh-ed25519 AAAA alice"
	if err = system.appendAuthorizedLine(context.Background(), keyFile, line); err != nil {
		t.Fatal(err)
	}
	if !hasCall(runner.calls, "chown", "root:root", keyFile) {
		t.Fatalf("authorized key rewrite was not restored to root ownership: %#v", runner.calls)
	}
	contents, err := os.ReadFile(keyFile)
	if err != nil || !strings.Contains(string(contents), line) {
		t.Fatalf("authorized key contents = %q, %v", contents, err)
	}
}

func TestCreatePersonCleansPartialUserAfterChpasswdFailure(t *testing.T) {
	runner := &recordingRunner{failName: "chpasswd"}
	system := New(t.TempDir(), true)
	system.Runner = runner
	person := domain.Person{Username: "alice", Role: domain.RoleDeveloper, SSHPublicKey: "ssh-ed25519 AAAA alice"}
	if _, err := system.CreatePerson(context.Background(), person, "local"); err == nil {
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
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper, SSHPublicKey: "ssh-ed25519 AAAA alice"}
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
