package host

import (
	"context"
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
	if err := system.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice Example", Email: "alice@example.test", Role: domain.RoleDeveloper, SSHPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest alice"}
	tree := domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: person.ID, Name: "default", Branch: "people/alice", Path: filepath.Join(root, "demo", "worktrees", "alice")}
	if err := system.CreateWorktree(context.Background(), project, person, tree, "main"); err != nil {
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
	keys, err := os.ReadFile(filepath.Join(root, "demo", ".ssh", "authorized_keys"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(keys), "--actor alice --worktree "+tree.Path) {
		t.Fatalf("authorized_keys = %q", keys)
	}
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
