package observe

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
)

func TestParseGitStatus(t *testing.T) {
	worktree := domain.Worktree{ID: "w1", Branch: "people/alice"}
	status := ParseGitStatus(worktree, []byte("# branch.oid 1234567890abcdef\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +2 -1\n1 M. N... 100644 100644 100644 a b staged\n1 .M N... 100644 100644 100644 a b modified\n? new.txt\nu UU N... 100644 100644 100644 100644 a b c conflict\n"))
	if status.ShortCommit != "1234567890ab" || status.Branch != "main" || status.Upstream == nil || *status.Upstream != "origin/main" {
		t.Fatalf("unexpected branch summary: %#v", status)
	}
	if got, want := []uint64{status.Ahead, status.Behind, status.Staged, status.Modified, status.Untracked, status.Conflicted}, []uint64{2, 1, 1, 1, 1, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("counts=%v, want=%v", got, want)
	}
	if status.State != domain.WorktreeDirty {
		t.Fatalf("state=%q, want dirty", status.State)
	}
}

type recordingGitRunner struct {
	command string
	args    []string
	env     []string
	output  []byte
	err     error
}

func (r *recordingGitRunner) Output(_ context.Context, command string, args, env []string) ([]byte, error) {
	r.command, r.args, r.env = command, append([]string(nil), args...), append([]string(nil), env...)
	return r.output, r.err
}

func TestSystemGitInspectorUsesProjectIdentityAndReportsFailures(t *testing.T) {
	runner := &recordingGitRunner{output: []byte("# branch.oid abc\n")}
	inspector := NewSystemGitInspector(runner)
	inspector.UID = func() int { return 0 }
	inspector.Lookup = func(string) error { return nil }
	status := inspector.Inspect(context.Background(), domain.Project{UnixUser: "soda-p-demo"}, domain.Worktree{ID: "w1", Path: "/srv/demo", Branch: "main"})
	if runner.command != "runuser" || status.State != domain.WorktreeClean {
		t.Fatalf("expected project-identity runuser command, command=%q status=%#v", runner.command, status)
	}
	if !reflect.DeepEqual(runner.env, []string{"GIT_OPTIONAL_LOCKS=0", "LC_ALL=C"}) {
		t.Fatalf("unexpected Git environment: %v", runner.env)
	}
	runner.err = errors.New("git failed")
	status = inspector.Inspect(context.Background(), domain.Project{}, domain.Worktree{ID: "w1", Branch: "main"})
	if status.State != domain.WorktreeUnavailable || status.Error == nil || *status.Error != "git failed" {
		t.Fatalf("failure must be explicit unavailable status: %#v", status)
	}
}
