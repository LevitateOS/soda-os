package observe

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

const (
	HostInterval    = 5 * time.Second
	GitInterval     = 3 * time.Second
	SessionInterval = time.Second
)

var ErrJournalFormat = errors.New("OpenSSH journal format is not recognized")

// ProjectStore is the narrow read-only daemon boundary required by observers.
// The daemon supplies this interface; observe does not import daemon packages.
type ProjectStore interface {
	ListProjects(context.Context) ([]domain.Project, error)
	ListPeople(context.Context) ([]domain.Person, error)
	ListSSHDeviceKeys(context.Context) ([]domain.SSHDeviceKey, error)
	ListWorktrees(context.Context, string) ([]domain.Worktree, error)
}

type HostSampler interface {
	SampleHost(context.Context) (domain.HostStatus, error)
}

type GitInspector interface {
	Inspect(context.Context, domain.Project, domain.Worktree) domain.WorktreeStatus
}

type SessionInspector interface {
	Inspect(context.Context, []domain.Project, []domain.Person, []domain.SSHDeviceKey, []domain.Worktree) (SessionObservation, error)
}

// SessionObservation can carry usable data and degraded telemetry together.
// A fatal format error is returned separately and preserves the prior snapshot.
type SessionObservation struct {
	Connections []domain.ActiveSSHConnection
	Degraded    bool
}

type Dependencies struct {
	Store    ProjectStore
	Host     HostSampler
	Git      GitInspector
	Sessions SessionInspector
	Broker   *Broker
}

// Manager owns sampling state. It performs no mutation outside its own memory.
type Manager struct {
	store    ProjectStore
	host     HostSampler
	git      GitInspector
	sessions SessionInspector
	broker   *Broker

	mu             sync.RWMutex
	hostStatus     domain.HostStatus
	worktreeStatus map[string][]domain.WorktreeStatus
	activeSessions []domain.ActiveSSHConnection
	sshState       domain.RuntimeState
	gitState       domain.RuntimeState
	interests      map[string]int
}

// Interest keeps Git sampling enabled for a project while a caller is reading
// its event stream. Close is idempotent.
type Interest struct {
	once sync.Once
	stop func()
}

func (i *Interest) Close() { i.once.Do(i.stop) }

func NewManager(deps Dependencies) (*Manager, error) {
	if deps.Store == nil || deps.Host == nil || deps.Git == nil || deps.Sessions == nil {
		return nil, errors.New("observe dependencies must all be provided")
	}
	if deps.Broker == nil {
		deps.Broker = NewBroker()
	}
	return &Manager{
		store:          deps.Store,
		host:           deps.Host,
		git:            deps.Git,
		sessions:       deps.Sessions,
		broker:         deps.Broker,
		worktreeStatus: make(map[string][]domain.WorktreeStatus),
		sshState:       domain.RuntimeUnavailable,
		gitState:       domain.RuntimeUnavailable,
		interests:      make(map[string]int),
	}, nil
}

func (m *Manager) Broker() *Broker { return m.broker }

func (m *Manager) Subscribe(ctx context.Context, projectID string) (*Subscription, *Interest) {
	interest := m.Interest(projectID)
	go func() {
		<-ctx.Done()
		interest.Close()
	}()
	return m.broker.Subscribe(ctx, projectID), interest
}

func (m *Manager) Interest(projectID string) *Interest {
	if projectID == "" {
		return &Interest{stop: func() {}}
	}
	m.mu.Lock()
	m.interests[projectID]++
	m.mu.Unlock()
	return &Interest{stop: func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.interests[projectID] <= 1 {
			delete(m.interests, projectID)
			return
		}
		m.interests[projectID]--
	}}
}

func (m *Manager) Interested(projectID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.interests[projectID] > 0
}

func (m *Manager) HostStatus() domain.HostStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneHost(m.hostStatus)
}

func (m *Manager) WorktreeStatuses(projectID string) []domain.WorktreeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.WorktreeStatus(nil), m.worktreeStatus[projectID]...)
}

func (m *Manager) ActiveSessions() []domain.ActiveSSHConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.ActiveSSHConnection(nil), m.activeSessions...)
}

// Run immediately samples all sources, then performs the specified periodic
// work until ctx is canceled. It is safe to call only once per manager.
func (m *Manager) Run(ctx context.Context) {
	m.RefreshHost(ctx)
	m.RefreshGit(ctx)
	m.RefreshSessions(ctx)
	go m.runEvery(ctx, HostInterval, m.RefreshHost)
	go m.runEvery(ctx, GitInterval, m.RefreshGit)
	go m.runEvery(ctx, SessionInterval, m.RefreshSessions)
}

func (m *Manager) runEvery(ctx context.Context, interval time.Duration, refresh func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh(ctx)
		}
	}
}

func (m *Manager) RefreshHost(ctx context.Context) {
	status, err := m.host.SampleHost(ctx)
	if err != nil {
		status = domain.HostStatus{SampledAt: time.Now(), Overall: domain.RuntimeUnavailable}
	}
	m.mu.Lock()
	status.SSHObserver = m.sshState
	status.GitObserver = m.gitState
	if status.Overall == domain.RuntimeReady && (m.sshState != domain.RuntimeReady || m.gitState != domain.RuntimeReady) {
		status.Overall = domain.RuntimeDegraded
	}
	changed := !reflect.DeepEqual(hostComparable(status), hostComparable(m.hostStatus))
	m.hostStatus = status
	m.mu.Unlock()
	if changed {
		m.broker.Publish(domain.EventHostChanged, "")
	}
}

func (m *Manager) RefreshGit(ctx context.Context) {
	m.mu.RLock()
	interested := make([]string, 0, len(m.interests))
	for id := range m.interests {
		interested = append(interested, id)
	}
	m.mu.RUnlock()
	if len(interested) == 0 {
		m.setGitState(domain.RuntimeReady)
		return
	}
	projects, err := m.store.ListProjects(ctx)
	if err != nil {
		m.setGitState(domain.RuntimeUnavailable)
		return
	}
	projectByID := make(map[string]domain.Project, len(projects))
	for _, project := range projects {
		projectByID[project.ID] = project
	}
	state := domain.RuntimeReady
	for _, id := range interested {
		project, ok := projectByID[id]
		if !ok {
			state = domain.RuntimeDegraded
			continue
		}
		worktrees, listErr := m.store.ListWorktrees(ctx, id)
		if listErr != nil {
			state = domain.RuntimeDegraded
			continue
		}
		statuses := make([]domain.WorktreeStatus, 0, len(worktrees))
		for _, worktree := range worktrees {
			status := m.git.Inspect(ctx, project, worktree)
			if status.State == domain.WorktreeUnavailable {
				state = domain.RuntimeDegraded
			}
			statuses = append(statuses, status)
		}
		m.mu.Lock()
		changed := !reflect.DeepEqual(m.worktreeStatus[id], statuses)
		m.worktreeStatus[id] = statuses
		m.mu.Unlock()
		if changed {
			m.broker.Publish(domain.EventGitChanged, id)
		}
	}
	m.setGitState(state)
}

func (m *Manager) RefreshSessions(ctx context.Context) {
	projects, err := m.store.ListProjects(ctx)
	if err != nil {
		m.setSSHState(domain.RuntimeUnavailable)
		return
	}
	people, err := m.store.ListPeople(ctx)
	if err != nil {
		m.setSSHState(domain.RuntimeUnavailable)
		return
	}
	keys, err := m.store.ListSSHDeviceKeys(ctx)
	if err != nil {
		m.setSSHState(domain.RuntimeUnavailable)
		return
	}
	var worktrees []domain.Worktree
	for _, project := range projects {
		items, listErr := m.store.ListWorktrees(ctx, project.ID)
		if listErr != nil {
			m.setSSHState(domain.RuntimeDegraded)
			return
		}
		worktrees = append(worktrees, items...)
	}
	observation, err := m.sessions.Inspect(ctx, projects, people, keys, worktrees)
	if err != nil {
		// An unknown journal format is intentionally not turned into an empty
		// connection list: the last known snapshot remains useful and honest.
		m.setSSHState(domain.RuntimeDegraded)
		return
	}
	nextSessions := append([]domain.ActiveSSHConnection(nil), observation.Connections...)
	m.mu.Lock()
	changed := !reflect.DeepEqual(m.activeSessions, nextSessions)
	previousState := m.sshState
	m.activeSessions = nextSessions
	m.sshState = domain.RuntimeReady
	if observation.Degraded {
		m.sshState = domain.RuntimeDegraded
	}
	stateChanged := previousState != m.sshState
	m.mu.Unlock()
	if changed {
		m.broker.Publish(domain.EventSessionsChanged, "")
	}
	if stateChanged {
		m.broker.Publish(domain.EventHostChanged, "")
	}
}

func (m *Manager) setGitState(state domain.RuntimeState) {
	m.mu.Lock()
	changed := m.gitState != state
	m.gitState = state
	m.mu.Unlock()
	if changed {
		m.broker.Publish(domain.EventHostChanged, "")
	}
}

func (m *Manager) setSSHState(state domain.RuntimeState) {
	m.mu.Lock()
	changed := m.sshState != state
	m.sshState = state
	m.mu.Unlock()
	if changed {
		m.broker.Publish(domain.EventHostChanged, "")
	}
}

func hostComparable(status domain.HostStatus) domain.HostStatus {
	status.SampledAt = time.Time{}
	return status
}

func cloneHost(status domain.HostStatus) domain.HostStatus {
	status.Services = append([]domain.ServiceStatus(nil), status.Services...)
	status.Interfaces = append([]domain.NetworkInterface(nil), status.Interfaces...)
	status.Filesystems = append([]domain.FilesystemStatus(nil), status.Filesystems...)
	return status
}

func unavailableWorktree(worktree domain.Worktree, err error) domain.WorktreeStatus {
	message := "Git status unavailable"
	if err != nil {
		message = err.Error()
	}
	return domain.WorktreeStatus{WorktreeID: worktree.ID, Branch: worktree.Branch, State: domain.WorktreeUnavailable, Error: &message}
}

func annotate(err error, label string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", label, err)
}
