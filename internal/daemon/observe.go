package daemon

import (
	"context"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/observe"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Observability adapts host status to sodad's gRPC service.
type Observability struct{ manager *observe.Manager }

func NewObservability(manager *observe.Manager) *Observability {
	return &Observability{manager: manager}
}

func (o *Observability) HostStatus(context.Context) (*sodav2.HostStatus, error) {
	return hostStatusProto(o.manager.HostStatus()), nil
}

func hostStatusProto(value domain.HostStatus) *sodav2.HostStatus {
	services := make([]*sodav2.ServiceStatus, 0, len(value.Health.Services))
	for _, item := range value.Health.Services {
		services = append(services, &sodav2.ServiceStatus{Name: item.Name, State: runtimeStateProto(item.State)})
	}
	interfaces := make([]*sodav2.NetworkInterface, 0, len(value.Network.Interfaces))
	for _, item := range value.Network.Interfaces {
		interfaces = append(interfaces, &sodav2.NetworkInterface{Name: item.Name, Addresses: append([]string(nil), item.Addresses...)})
	}
	filesystems := make([]*sodav2.FilesystemStatus, 0, len(value.Resources.Filesystems))
	for _, item := range value.Resources.Filesystems {
		filesystems = append(filesystems, &sodav2.FilesystemStatus{Path: item.Path, TotalBytes: item.TotalBytes, AvailableBytes: item.AvailableBytes})
	}
	result := &sodav2.HostStatus{SampledAt: timestamppb.New(value.SampledAt), Overall: runtimeStateProto(value.Health.Overall), Services: services, SshFirewallReady: value.Firewall.SSHReady, CockpitFirewallReady: value.Firewall.CockpitReady, Interfaces: interfaces, LoadAverage: &sodav2.LoadAverage{OneMinute: value.Resources.LoadAverage[0], FiveMinutes: value.Resources.LoadAverage[1], FifteenMinutes: value.Resources.LoadAverage[2]}, UptimeSeconds: value.Resources.UptimeSeconds, MemoryTotalBytes: value.Resources.MemoryTotalBytes, MemoryAvailableBytes: value.Resources.MemoryAvailableBytes, Filesystems: filesystems}
	result.CpuPercent = value.Resources.CPUPercent
	return result
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
