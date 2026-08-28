package domain

import "time"

type RuntimeState string

const (
	RuntimeReady       RuntimeState = "ready"
	RuntimeDegraded    RuntimeState = "degraded"
	RuntimeUnavailable RuntimeState = "unavailable"
)

type ServiceStatus struct {
	Name  string
	State RuntimeState
}

type NetworkInterface struct {
	Name      string
	Addresses []string
}

type FilesystemStatus struct {
	Path           string
	TotalBytes     uint64
	AvailableBytes uint64
}

type HostHealth struct {
	Overall  RuntimeState
	Services []ServiceStatus
}

type HostNetwork struct {
	Interfaces []NetworkInterface
}

type FirewallStatus struct {
	SSHReady     bool
	CockpitReady bool
}

type HostResources struct {
	CPUPercent           *float64
	LoadAverage          [3]float64
	UptimeSeconds        uint64
	MemoryTotalBytes     uint64
	MemoryAvailableBytes uint64
	Filesystems          []FilesystemStatus
}

type HostStatus struct {
	SampledAt time.Time
	Health    HostHealth
	Network   HostNetwork
	Firewall  FirewallStatus
	Resources HostResources
}
