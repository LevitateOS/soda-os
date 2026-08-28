package observe

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

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

func TestManagerStoresIndependentHostSnapshot(t *testing.T) {
	host := &fakeHost{status: domain.HostStatus{
		SampledAt: time.Now(),
		Health: domain.HostHealth{
			Overall:  domain.RuntimeReady,
			Services: []domain.ServiceStatus{{Name: "sshd", State: domain.RuntimeReady}},
		},
	}}
	manager, err := NewManager(host)
	if err != nil {
		t.Fatal(err)
	}
	manager.RefreshHost(context.Background())

	first := manager.HostStatus()
	first.Health.Services[0].State = domain.RuntimeUnavailable
	if got := manager.HostStatus().Health.Services[0].State; got != domain.RuntimeReady {
		t.Fatalf("stored host snapshot was mutated through a caller: %q", got)
	}
}

func TestManagerMarksSamplingFailureUnavailable(t *testing.T) {
	manager, err := NewManager(&fakeHost{err: errors.New("host unavailable")})
	if err != nil {
		t.Fatal(err)
	}
	manager.RefreshHost(context.Background())
	status := manager.HostStatus()
	if status.Health.Overall != domain.RuntimeUnavailable || status.SampledAt.IsZero() {
		t.Fatalf("failed host sample = %#v", status)
	}
}

func TestManagerRunSamplesImmediately(t *testing.T) {
	host := &fakeHost{status: domain.HostStatus{Health: domain.HostHealth{Overall: domain.RuntimeReady}}}
	manager, err := NewManager(host)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	manager.Run(ctx)
	cancel()

	host.mu.Lock()
	calls := host.calls
	host.mu.Unlock()
	if calls != 1 {
		t.Fatalf("initial host sample calls = %d", calls)
	}
}

func TestManagerRequiresHostSampler(t *testing.T) {
	if _, err := NewManager(nil); err == nil {
		t.Fatal("manager accepted a nil host sampler")
	}
}
