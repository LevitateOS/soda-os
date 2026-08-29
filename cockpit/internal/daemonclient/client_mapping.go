package daemonclient

import sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"

func person(value *sodav2.Person) Person {
	return Person{ID: value.GetId(), Username: value.GetUsername(), DisplayName: value.GetDisplayName(), Email: value.GetEmail(), Role: roleFromProto(value.GetRole())}
}
func sshDeviceKey(value *sodav2.SshDeviceKey) SSHDeviceKey {
	result := SSHDeviceKey{ID: value.GetId(), PersonID: value.GetPersonId(), Label: value.GetLabel(), PublicKey: value.GetPublicKey(), Fingerprint: value.GetFingerprint(), IdentityFileHint: value.GetIdentityFileHint()}
	if timestamp := value.GetCreatedAt(); timestamp != nil {
		result.CreatedAt = timestamp.AsTime()
	}
	return result
}
func people(values []*sodav2.Person) []Person {
	items := make([]Person, 0, len(values))
	for _, value := range values {
		items = append(items, person(value))
	}
	return items
}
func project(value *sodav2.Project) Project {
	return Project{ID: value.GetId(), Slug: value.GetSlug(), Name: value.GetName(), UnixUser: value.GetUnixUser(), Profile: profileFromProto(value.GetProfile()), Source: sourceFromProto(value.GetSource())}
}
func projects(values []*sodav2.Project) []Project {
	items := make([]Project, 0, len(values))
	for _, value := range values {
		items = append(items, project(value))
	}
	return items
}
func worktree(value *sodav2.Worktree) Worktree {
	return Worktree{ID: value.GetId(), ProjectID: value.GetProjectId(), PersonID: value.GetPersonId(), Name: value.GetName(), Branch: value.GetBranch(), Path: value.GetPath()}
}
func job(value *sodav2.ProvisioningJob) ProvisioningJob {
	result := ProvisioningJob{ID: value.GetId(), ProjectID: value.GetProjectId(), State: jobState(value.GetState())}
	if value.Error != nil {
		result.Error = value.Error
	}
	return result
}
func toolchain(value *sodav2.ToolchainInstallation) ToolchainInstallation {
	return ToolchainInstallation{ID: value.GetId(), Profile: profileFromProto(value.GetProfile()), Version: value.GetVersion(), Path: value.GetPath(), Checksum: value.GetChecksum(), State: jobState(value.GetState())}
}
func osUpdateStatus(value *sodav2.OSUpdateStatus) OSUpdateStatus {
	if value == nil {
		return OSUpdateStatus{}
	}
	result := OSUpdateStatus{ReadOnly: value.GetReadOnly()}
	if value.Booted != nil {
		result.Booted = osDeployment(value.Booted)
	}
	if value.Staged != nil {
		result.Staged = osDeployment(value.Staged)
	}
	return result
}
func osDeployment(value *sodav2.OSDeployment) *OSDeployment {
	return &OSDeployment{
		ImageReference: value.GetImageReference(), Version: value.GetVersion(), Digest: value.GetDigest(),
		Architecture: value.GetArchitecture(),
		Incompatible: value.GetIncompatible(), DownloadOnly: value.GetDownloadOnly(),
	}
}
func hostStatus(value *sodav2.HostStatus) HostStatus {
	result := HostStatus{
		Health:    HostHealth{Overall: runtimeState(value.GetOverall())},
		Firewall:  FirewallStatus{SSHReady: value.GetSshFirewallReady(), CockpitReady: value.GetCockpitFirewallReady()},
		Resources: HostResources{UptimeSeconds: value.GetUptimeSeconds(), MemoryTotalBytes: value.GetMemoryTotalBytes(), MemoryAvailableBytes: value.GetMemoryAvailableBytes()},
	}
	if timestamp := value.GetSampledAt(); timestamp != nil {
		result.SampledAt = timestamp.AsTime()
	}
	if value.CpuPercent != nil {
		cpu := value.GetCpuPercent()
		result.Resources.CPUPercent = &cpu
	}
	if load := value.GetLoadAverage(); load != nil {
		result.Resources.LoadAverage = [3]float64{load.GetOneMinute(), load.GetFiveMinutes(), load.GetFifteenMinutes()}
	}
	for _, item := range value.GetServices() {
		result.Health.Services = append(result.Health.Services, ServiceStatus{Name: item.GetName(), State: runtimeState(item.GetState())})
	}
	for _, item := range value.GetInterfaces() {
		result.Network.Interfaces = append(result.Network.Interfaces, NetworkInterface{Name: item.GetName(), Addresses: item.GetAddresses()})
	}
	for _, item := range value.GetFilesystems() {
		result.Resources.Filesystems = append(result.Resources.Filesystems, FilesystemStatus{Path: item.GetPath(), TotalBytes: item.GetTotalBytes(), AvailableBytes: item.GetAvailableBytes()})
	}
	return result
}
func roleToProto(value Role) sodav2.Role {
	if value == RoleAdmin {
		return sodav2.Role_ROLE_ADMIN
	}
	return sodav2.Role_ROLE_DEVELOPER
}
func roleFromProto(value sodav2.Role) Role {
	if value == sodav2.Role_ROLE_ADMIN {
		return RoleAdmin
	}
	return RoleDeveloper
}
func profileToProto(value string) sodav2.ToolchainProfile {
	switch value {
	case "web":
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_WEB
	case "python":
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_PYTHON
	case "rust":
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_RUST
	case "go":
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO
	default:
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_UNSPECIFIED
	}
}
func profileFromProto(value sodav2.ToolchainProfile) ToolchainProfile {
	switch value {
	case sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_WEB:
		return "web"
	case sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_PYTHON:
		return "python"
	case sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_RUST:
		return "rust"
	case sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO:
		return "go"
	default:
		return ""
	}
}
func sourceToProto(value ProjectSource) *sodav2.ProjectSource {
	if git, ok := value.(GitProjectSource); ok {
		return &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Git{Git: &sodav2.GitProjectSource{RemoteUrl: git.RemoteURL}}}
	}
	return &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}
}
func sourceFromProto(value *sodav2.ProjectSource) ProjectSource {
	if git := value.GetGit(); git != nil {
		return GitProjectSource{RemoteURL: git.GetRemoteUrl()}
	}
	return EmptyProjectSource{}
}
func jobState(value sodav2.JobState) JobState {
	switch value {
	case sodav2.JobState_JOB_STATE_INSTALLING:
		return "installing"
	case sodav2.JobState_JOB_STATE_READY:
		return "ready"
	case sodav2.JobState_JOB_STATE_FAILED:
		return "failed"
	default:
		return ""
	}
}
func runtimeState(value sodav2.RuntimeState) RuntimeState {
	switch value {
	case sodav2.RuntimeState_RUNTIME_STATE_READY:
		return "ready"
	case sodav2.RuntimeState_RUNTIME_STATE_DEGRADED:
		return "degraded"
	case sodav2.RuntimeState_RUNTIME_STATE_UNAVAILABLE:
		return "unavailable"
	default:
		return ""
	}
}
