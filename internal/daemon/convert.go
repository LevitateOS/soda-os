package daemon

import (
	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func personProto(v domain.Person) *sodav2.Person {
	return &sodav2.Person{Id: v.ID, Username: v.Username, DisplayName: v.DisplayName, Email: v.Email, Role: roleProto(v.Role)}
}
func gitIdentityProto(v domain.GitIdentity) *sodav2.GitIdentity {
	return &sodav2.GitIdentity{PersonId: v.PersonID, PublicKey: v.PublicKey, Fingerprint: v.Fingerprint}
}
func sshDeviceKeyProto(v domain.SSHDeviceKey) *sodav2.SshDeviceKey {
	return &sodav2.SshDeviceKey{Id: v.ID, PersonId: v.PersonID, Label: v.Label, PublicKey: v.PublicKey, Fingerprint: v.Fingerprint, IdentityFileHint: v.IdentityFileHint, CreatedAt: timestamppb.New(v.CreatedAt)}
}
func roleProto(v domain.Role) sodav2.Role {
	if v == domain.RoleAdmin {
		return sodav2.Role_ROLE_ADMIN
	}
	if v == domain.RoleDeveloper {
		return sodav2.Role_ROLE_DEVELOPER
	}
	return sodav2.Role_ROLE_UNSPECIFIED
}
func roleDomain(v sodav2.Role) (domain.Role, error) {
	switch v {
	case sodav2.Role_ROLE_ADMIN:
		return domain.RoleAdmin, nil
	case sodav2.Role_ROLE_DEVELOPER:
		return domain.RoleDeveloper, nil
	default:
		return "", invalid("role is required")
	}
}
func profileProto(v domain.ToolchainProfile) sodav2.ToolchainProfile {
	switch v {
	case domain.ToolchainWeb:
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_WEB
	case domain.ToolchainPython:
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_PYTHON
	case domain.ToolchainRust:
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_RUST
	case domain.ToolchainGo:
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO
	default:
		return sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_UNSPECIFIED
	}
}
func profileDomain(v sodav2.ToolchainProfile) (domain.ToolchainProfile, error) {
	switch v {
	case sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_WEB:
		return domain.ToolchainWeb, nil
	case sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_PYTHON:
		return domain.ToolchainPython, nil
	case sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_RUST:
		return domain.ToolchainRust, nil
	case sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO:
		return domain.ToolchainGo, nil
	default:
		return "", invalid("toolchain profile is required")
	}
}
func sourceDomain(v *sodav2.ProjectSource) (domain.ProjectSource, error) {
	if v == nil {
		return nil, invalid("project source is required")
	}
	switch source := v.Source.(type) {
	case *sodav2.ProjectSource_Empty:
		return domain.EmptyProjectSource{}, nil
	case *sodav2.ProjectSource_Git:
		value := domain.GitProjectSource{RemoteURL: source.Git.GetRemoteUrl()}
		if err := domain.ValidateProjectSource(value); err != nil {
			return nil, invalid("%v", err)
		}
		return value, nil
	default:
		return nil, invalid("project source must be empty or Git")
	}
}
func sourceProto(v domain.ProjectSource) *sodav2.ProjectSource {
	switch source := v.(type) {
	case domain.EmptyProjectSource:
		return &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}
	case domain.GitProjectSource:
		return &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Git{Git: &sodav2.GitProjectSource{RemoteUrl: source.RemoteURL}}}
	default:
		return nil
	}
}
func projectProto(v domain.Project) *sodav2.Project {
	return &sodav2.Project{Id: v.ID, Slug: v.Slug, Name: v.Name, UnixUser: v.UnixUser, Profile: profileProto(v.Profile), Source: sourceProto(v.Source), BootstrapPersonId: v.BootstrapPersonID}
}
func membershipProto(v domain.Membership) *sodav2.Membership {
	return &sodav2.Membership{ProjectId: v.ProjectID, PersonId: v.PersonID}
}
func worktreeProto(v domain.Worktree) *sodav2.Worktree {
	return &sodav2.Worktree{Id: v.ID, ProjectId: v.ProjectID, PersonId: v.PersonID, Name: v.Name, Branch: v.Branch, Path: v.Path}
}
func stateProto(v domain.JobState) sodav2.JobState {
	switch v {
	case domain.JobInstalling:
		return sodav2.JobState_JOB_STATE_INSTALLING
	case domain.JobReady:
		return sodav2.JobState_JOB_STATE_READY
	case domain.JobFailed:
		return sodav2.JobState_JOB_STATE_FAILED
	default:
		return sodav2.JobState_JOB_STATE_UNSPECIFIED
	}
}
func jobProto(v domain.ProvisioningJob) *sodav2.ProvisioningJob {
	return &sodav2.ProvisioningJob{Id: v.ID, ProjectId: v.ProjectID, State: stateProto(v.State), Error: v.Error}
}
func installationProto(v domain.ToolchainInstallation) *sodav2.ToolchainInstallation {
	return &sodav2.ToolchainInstallation{Id: v.ID, Profile: profileProto(v.Profile), Version: v.Version, Path: v.Path, Checksum: v.Checksum, State: stateProto(v.State)}
}
func resolutionProto(v domain.ProjectToolchainResolution) *sodav2.ProjectToolchainResolution {
	return &sodav2.ProjectToolchainResolution{ProjectId: v.ProjectID, ToolchainInstallationId: v.ToolchainInstallationID}
}
