package observe

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

type fakeStore struct {
	projects   []domain.Project
	people     []domain.Person
	keys       []domain.SSHDeviceKey
	worktrees  map[string][]domain.Worktree
	projectErr error
	peopleErr  error
	treeErr    error
}

func (s fakeStore) ListProjects(context.Context) ([]domain.Project, error) {
	return s.projects, s.projectErr
}
func (s fakeStore) ListPeople(context.Context) ([]domain.Person, error) { return s.people, s.peopleErr }
func (s fakeStore) ListSSHDeviceKeys(context.Context) ([]domain.SSHDeviceKey, error) {
	return s.keys, nil
}
func (s fakeStore) ListWorktrees(_ context.Context, id string) ([]domain.Worktree, error) {
	return s.worktrees[id], s.treeErr
}

type fakeHost struct {
	mu     sync.Mutex
	status domain.HostStatus
	err    error
	calls  int
}

func (f *fakeHost) SampleHost(context.Context) (domain.HostStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.status, f.err
}

type fakeGit struct {
	mu     sync.Mutex
	status domain.WorktreeStatus
	calls  int
}

func (f *fakeGit) Inspect(_ context.Context, _ domain.Project, tree domain.Worktree) domain.WorktreeStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	result := f.status
	result.WorktreeID = tree.ID
	return result
}

type fakeSessions struct {
	observation SessionObservation
	err         error
	calls       int
}

func (f *fakeSessions) Inspect(context.Context, []domain.Project, []domain.Person, []domain.SSHDeviceKey, []domain.Worktree) (SessionObservation, error) {
	f.calls++
	return f.observation, f.err
}

func TestManagerInterestLimitsGitSamplingAndCancellation(t *testing.T) {
	project := domain.Project{ID: "p1", UnixUser: "soda-p-demo"}
	tree := domain.Worktree{ID: "w1", ProjectID: "p1", Branch: "main"}
	host := &fakeHost{status: domain.HostStatus{Overall: domain.RuntimeReady}}
	git := &fakeGit{status: domain.WorktreeStatus{State: domain.WorktreeClean}}
	manager := managerFor(t, fakeStore{projects: []domain.Project{project}, worktrees: map[string][]domain.Worktree{"p1": {tree}}}, host, git, &fakeSessions{})
	manager.RefreshGit(context.Background())
	if git.calls != 0 {
		t.Fatal("Git sampling must not run without interested subscribers")
	}
	interest := manager.Interest("p1")
	manager.RefreshGit(context.Background())
	if git.calls != 1 || len(manager.WorktreeStatuses("p1")) != 1 {
		t.Fatalf("expected one interested Git inspection, calls=%d", git.calls)
	}
	interest.Close()
	manager.RefreshGit(context.Background())
	if git.calls != 1 || manager.Interested("p1") {
		t.Fatal("closed interest must stop subsequent Git sampling")
	}
}

func TestManagerSubscribeReleasesInterestOnContextCancellation(t *testing.T) {
	manager := managerFor(t, fakeStore{}, &fakeHost{}, &fakeGit{}, &fakeSessions{})
	ctx, cancel := context.WithCancel(context.Background())
	subscription, _ := manager.Subscribe(ctx, "p1")
	<-subscription.C
	if !manager.Interested("p1") {
		t.Fatal("project subscription should register interest")
	}
	cancel()
	deadline := time.After(time.Second)
	for manager.Interested("p1") {
		select {
		case <-deadline:
			t.Fatal("context cancellation did not release project interest")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestManagerPreservesSessionsOnObserverFailure(t *testing.T) {
	sessions := &fakeSessions{observation: SessionObservation{Connections: []domain.ActiveSSHConnection{{ID: "live"}}}}
	manager := managerFor(t, fakeStore{}, &fakeHost{status: domain.HostStatus{Overall: domain.RuntimeReady}}, &fakeGit{}, sessions)
	manager.RefreshSessions(context.Background())
	if got := manager.ActiveSessions(); len(got) != 1 {
		t.Fatalf("expected initial session, got %#v", got)
	}
	sessions.err = ErrJournalFormat
	manager.RefreshSessions(context.Background())
	if got := manager.ActiveSessions(); len(got) != 1 || got[0].ID != "live" {
		t.Fatalf("observer failure must retain prior sessions, got %#v", got)
	}
	manager.RefreshHost(context.Background())
	if got := manager.HostStatus(); got.SSHObserver != domain.RuntimeDegraded || got.Overall != domain.RuntimeDegraded {
		t.Fatalf("observer failure must be visible as degraded host telemetry: %#v", got)
	}
}

func TestManagerHostPublishesOnlyOnRenderedChange(t *testing.T) {
	host := &fakeHost{status: domain.HostStatus{SampledAt: time.Now(), Overall: domain.RuntimeReady}}
	manager := managerFor(t, fakeStore{}, host, &fakeGit{}, &fakeSessions{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subscription := manager.Broker().Subscribe(ctx, "")
	<-subscription.C
	manager.RefreshHost(ctx)
	<-subscription.C
	manager.RefreshHost(ctx)
	select {
	case message := <-subscription.C:
		t.Fatalf("same rendered host snapshot should not emit event: %#v", message)
	case <-time.After(CoalesceInterval + 100*time.Millisecond):
	}
	host.mu.Lock()
	host.status.CockpitFirewallReady = true
	host.mu.Unlock()
	manager.RefreshHost(ctx)
	if message := receive(t, subscription.C); message.Event == nil || message.Event.Kind != domain.EventHostChanged {
		t.Fatalf("changed host must emit host_changed: %#v", message)
	}
}

func TestManagerDependencyFailuresAreDegraded(t *testing.T) {
	manager := managerFor(t, fakeStore{projectErr: errors.New("database unavailable")}, &fakeHost{status: domain.HostStatus{}}, &fakeGit{}, &fakeSessions{})
	manager.Interest("p1")
	manager.RefreshGit(context.Background())
	manager.RefreshHost(context.Background())
	if got := manager.HostStatus().GitObserver; got != domain.RuntimeUnavailable {
		t.Fatalf("expected unavailable Git observer, got %q", got)
	}
}

func managerFor(t *testing.T, store ProjectStore, host HostSampler, git GitInspector, sessions SessionInspector) *Manager {
	t.Helper()
	manager, err := NewManager(Dependencies{Store: store, Host: host, Git: git, Sessions: sessions, Broker: NewBroker()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Broker().Close() })
	return manager
}
