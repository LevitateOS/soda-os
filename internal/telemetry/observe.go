package telemetry

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

const HostInterval = 5 * time.Second

type HostSampler interface {
	SampleHost(context.Context) (domain.HostStatus, error)
}

// Manager owns the current host-status sample in memory.
type Manager struct {
	host HostSampler

	mu         sync.RWMutex
	hostStatus domain.HostStatus
}

func NewManager(host HostSampler) (*Manager, error) {
	if host == nil {
		return nil, errors.New("host sampler must be provided")
	}
	return &Manager{host: host}, nil
}

func (m *Manager) HostStatus() domain.HostStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneHost(m.hostStatus)
}

// Run immediately samples the host, then refreshes it until ctx is canceled.
func (m *Manager) Run(ctx context.Context) {
	m.RefreshHost(ctx)
	go m.runEvery(ctx, HostInterval, m.RefreshHost)
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
		status = domain.HostStatus{SampledAt: time.Now(), Health: domain.HostHealth{Overall: domain.RuntimeUnavailable}}
	}
	m.mu.Lock()
	m.hostStatus = status
	m.mu.Unlock()
}

func cloneHost(status domain.HostStatus) domain.HostStatus {
	status.Health.Services = append([]domain.ServiceStatus(nil), status.Health.Services...)
	status.Network.Interfaces = append([]domain.NetworkInterface(nil), status.Network.Interfaces...)
	status.Resources.Filesystems = append([]domain.FilesystemStatus(nil), status.Resources.Filesystems...)
	return status
}
