package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/store"
)

type fakeHost struct {
	mu          sync.Mutex
	people      personEvents
	workspaces  workspaceEvents
	access      accessEvents
	environment int
}

type personEvents struct {
	created  int
	imported int
	cleanups int
}

type workspaceEvents struct {
	projects         []domain.Project
	worktrees        []domain.Worktree
	projectCleanups  int
	worktreeCleanups int
	attempts         int
	failAt           int
	baseRefs         []string
}

type accessEvents struct {
	reconciliations [][]domain.ProjectAccess
	err             error
}

func (h *fakeHost) CreatePerson(context.Context, domain.Person, string) (host.Cleanup, error) {
	h.mu.Lock()
	h.people.created++
	h.mu.Unlock()
	return func(context.Context) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.people.cleanups++
		return nil
	}, nil
}
func (h *fakeHost) ImportPerson(context.Context, domain.Person) (host.Cleanup, error) {
	h.mu.Lock()
	h.people.imported++
	h.mu.Unlock()
	return func(context.Context) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.people.cleanups++
		return nil
	}, nil
}
func (h *fakeHost) CreateProject(_ context.Context, value domain.Project) (host.Cleanup, error) {
	h.mu.Lock()
	h.workspaces.projects = append(h.workspaces.projects, value)
	h.mu.Unlock()
	return func(context.Context) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.workspaces.projectCleanups++
		return nil
	}, nil
}
func (*fakeHost) EnsureRepository(context.Context, domain.Project) error        { return nil }
func (*fakeHost) DefaultBranch(context.Context, domain.Project) (string, error) { return "trunk", nil }
func (h *fakeHost) CreateWorktree(_ context.Context, _ domain.Project, _ domain.Person, value domain.Worktree, baseRef string) (host.Cleanup, error) {
	h.mu.Lock()
	h.workspaces.attempts++
	if h.workspaces.failAt == h.workspaces.attempts {
		h.mu.Unlock()
		return nil, errors.New("injected personal workspace failure")
	}
	h.workspaces.worktrees = append(h.workspaces.worktrees, value)
	h.workspaces.baseRefs = append(h.workspaces.baseRefs, baseRef)
	h.mu.Unlock()
	return func(context.Context) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.workspaces.worktreeCleanups++
		return nil
	}, nil
}
func (h *fakeHost) ReconcileAuthorizedKeys(_ context.Context, _ domain.Project, access []domain.ProjectAccess) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	copyOfAccess := append([]domain.ProjectAccess(nil), access...)
	h.access.reconciliations = append(h.access.reconciliations, copyOfAccess)
	return h.access.err
}
func (h *fakeHost) WriteProjectEnvironment(context.Context, domain.Project, string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.environment++
	return nil
}
func (*fakeHost) DeployPublicKey(context.Context, domain.Project) (string, error) {
	return "ssh-ed25519 AAAA project", nil
}

type fakeInstaller struct{}

func (fakeInstaller) Install(_ context.Context, profile domain.ToolchainProfile) (domain.ToolchainInstallation, error) {
	return domain.ToolchainInstallation{Profile: profile, Version: string(profile) + "=test", Path: "/toolchains/" + string(profile), Checksum: "verified", State: domain.JobReady}, nil
}

type blockingInstaller struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

type timeoutInstaller struct{}

func (timeoutInstaller) Install(ctx context.Context, _ domain.ToolchainProfile) (domain.ToolchainInstallation, error) {
	<-ctx.Done()
	return domain.ToolchainInstallation{}, ctx.Err()
}

func (b *blockingInstaller) Install(_ context.Context, profile domain.ToolchainProfile) (domain.ToolchainInstallation, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return domain.ToolchainInstallation{Profile: profile, Version: string(profile) + "=blocked", Path: "/toolchains/" + string(profile), Checksum: "verified", State: domain.JobReady}, nil
}

type observeHost struct{}

func (observeHost) SampleHost(context.Context) (domain.HostStatus, error) {
	return domain.HostStatus{SampledAt: time.Now(), Health: domain.HostHealth{Overall: domain.RuntimeReady}}, nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: fakeInstaller{}, ProjectsRoot: filepath.Join(t.TempDir(), "projects")})
	t.Cleanup(service.Close)
	return service
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	return repository
}
