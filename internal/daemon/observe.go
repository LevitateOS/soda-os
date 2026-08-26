package daemon

import (
	"context"
	"sync"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/observe"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Observability adapts the transport-neutral observer to sodad's gRPC service.
type Observability struct{ manager *observe.Manager }

func NewObservability(manager *observe.Manager) *Observability {
	return &Observability{manager: manager}
}

func (o *Observability) Publish(kind domain.EventKind, projectID *string) {
	project := ""
	if projectID != nil {
		project = *projectID
	}
	o.manager.Broker().Publish(kind, project)
}

func (o *Observability) HostStatus(context.Context) (*sodav2.HostStatus, error) {
	return hostStatusProto(o.manager.HostStatus()), nil
}

func (o *Observability) WorktreeStatuses(_ context.Context, projectID string) ([]*sodav2.WorktreeStatus, error) {
	values := o.manager.WorktreeStatuses(projectID)
	result := make([]*sodav2.WorktreeStatus, 0, len(values))
	for _, value := range values {
		result = append(result, worktreeStatusProto(value))
	}
	return result, nil
}

func (o *Observability) ActiveSSHConnections(context.Context) ([]*sodav2.ActiveSshConnection, error) {
	values := o.manager.ActiveSessions()
	result := make([]*sodav2.ActiveSshConnection, 0, len(values))
	for _, value := range values {
		result = append(result, activeConnectionProto(value))
	}
	return result, nil
}

func (o *Observability) Subscribe(ctx context.Context, projectID *string) (EventSubscription, error) {
	project := ""
	if projectID != nil {
		project = *projectID
	}
	subscription, interest := o.manager.Subscribe(ctx, project)
	adapter := &observeSubscription{subscription: subscription, interest: interest, messages: make(chan EventMessage, observe.BrokerCapacity), done: make(chan struct{})}
	go adapter.forward(ctx)
	return adapter, nil
}

type observeSubscription struct {
	subscription *observe.Subscription
	interest     *observe.Interest
	messages     chan EventMessage
	done         chan struct{}
	once         sync.Once
}

func (s *observeSubscription) Messages() <-chan EventMessage { return s.messages }
func (s *observeSubscription) Close() {
	s.once.Do(func() { close(s.done); s.subscription.Cancel(); s.interest.Close() })
}
func (s *observeSubscription) forward(ctx context.Context) {
	defer close(s.messages)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case message, ok := <-s.subscription.C:
			if !ok {
				return
			}
			select {
			case s.messages <- EventMessage{Event: message.Event, Refresh: message.Refresh}:
			case <-ctx.Done():
				return
			case <-s.done:
				return
			}
		}
	}
}

func hostStatusProto(value domain.HostStatus) *sodav2.HostStatus {
	services := make([]*sodav2.ServiceStatus, 0, len(value.Services))
	for _, item := range value.Services {
		services = append(services, &sodav2.ServiceStatus{Name: item.Name, State: runtimeStateProto(item.State)})
	}
	interfaces := make([]*sodav2.NetworkInterface, 0, len(value.Interfaces))
	for _, item := range value.Interfaces {
		interfaces = append(interfaces, &sodav2.NetworkInterface{Name: item.Name, Addresses: append([]string(nil), item.Addresses...)})
	}
	filesystems := make([]*sodav2.FilesystemStatus, 0, len(value.Filesystems))
	for _, item := range value.Filesystems {
		filesystems = append(filesystems, &sodav2.FilesystemStatus{Path: item.Path, TotalBytes: item.TotalBytes, AvailableBytes: item.AvailableBytes})
	}
	result := &sodav2.HostStatus{SampledAt: timestamppb.New(value.SampledAt), Overall: runtimeStateProto(value.Overall), Services: services, SshFirewallReady: value.SSHFirewallReady, CockpitFirewallReady: value.CockpitFirewallReady, Interfaces: interfaces, LoadAverage: &sodav2.LoadAverage{OneMinute: value.LoadAverage[0], FiveMinutes: value.LoadAverage[1], FifteenMinutes: value.LoadAverage[2]}, UptimeSeconds: value.UptimeSeconds, MemoryTotalBytes: value.MemoryTotalBytes, MemoryAvailableBytes: value.MemoryAvailableBytes, Filesystems: filesystems, SshObserver: runtimeStateProto(value.SSHObserver), GitObserver: runtimeStateProto(value.GitObserver)}
	result.CpuPercent = value.CPUPercent
	return result
}
func worktreeStatusProto(value domain.WorktreeStatus) *sodav2.WorktreeStatus {
	return &sodav2.WorktreeStatus{WorktreeId: value.WorktreeID, Branch: value.Branch, ShortCommit: value.ShortCommit, Upstream: value.Upstream, Ahead: value.Ahead, Behind: value.Behind, Staged: value.Staged, Modified: value.Modified, Untracked: value.Untracked, Conflicted: value.Conflicted, State: worktreeStateProto(value.State), Error: value.Error}
}
func activeConnectionProto(value domain.ActiveSSHConnection) *sodav2.ActiveSshConnection {
	channels := make([]*sodav2.SshChannel, 0, len(value.Channels))
	for _, channel := range value.Channels {
		channels = append(channels, &sodav2.SshChannel{Kind: sshChannelProto(channel.Kind), WorktreeId: channel.WorktreeID})
	}
	return &sodav2.ActiveSshConnection{Id: value.ID, ProjectId: value.ProjectID, PersonId: value.PersonID, ConnectedAt: timestamppb.New(value.ConnectedAt), ClientAddress: value.ClientAddress, ClientPort: uint32(value.ClientPort), ServerAddress: value.ServerAddress, ServerPort: uint32(value.ServerPort), Channels: channels}
}
func runtimeStateProto(value domain.RuntimeState) sodav2.RuntimeState {
	switch value {
	case domain.RuntimeReady:
		return sodav2.RuntimeState_RUNTIME_STATE_READY
	case domain.RuntimeDegraded:
		return sodav2.RuntimeState_RUNTIME_STATE_DEGRADED
	case domain.RuntimeUnavailable:
		return sodav2.RuntimeState_RUNTIME_STATE_UNAVAILABLE
	default:
		return sodav2.RuntimeState_RUNTIME_STATE_UNSPECIFIED
	}
}
func worktreeStateProto(value domain.WorktreeState) sodav2.WorktreeState {
	switch value {
	case domain.WorktreeClean:
		return sodav2.WorktreeState_WORKTREE_STATE_CLEAN
	case domain.WorktreeDirty:
		return sodav2.WorktreeState_WORKTREE_STATE_DIRTY
	case domain.WorktreeUnavailable:
		return sodav2.WorktreeState_WORKTREE_STATE_UNAVAILABLE
	default:
		return sodav2.WorktreeState_WORKTREE_STATE_UNSPECIFIED
	}
}
func sshChannelProto(value domain.SSHChannelKind) sodav2.SshChannelKind {
	switch value {
	case domain.SSHChannelInteractive:
		return sodav2.SshChannelKind_SSH_CHANNEL_KIND_INTERACTIVE
	case domain.SSHChannelCommand:
		return sodav2.SshChannelKind_SSH_CHANNEL_KIND_COMMAND
	case domain.SSHChannelSFTP:
		return sodav2.SshChannelKind_SSH_CHANNEL_KIND_SFTP
	default:
		return sodav2.SshChannelKind_SSH_CHANNEL_KIND_UNSPECIFIED
	}
}
