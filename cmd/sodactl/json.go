package main

import (
	"strings"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
)

func healthJSON(health *sodav2.HealthResponse) any {
	if health == nil {
		return nil
	}
	return map[string]any{"status": health.Status, "service": health.Service, "version": health.Version}
}
func personJSON(person *sodav2.Person) any {
	if person == nil {
		return nil
	}
	return map[string]any{"id": person.Id, "username": person.Username, "display_name": person.DisplayName, "email": person.Email, "role": roleName(person.Role)}
}
func peopleJSON(people []*sodav2.Person) []any {
	return mapJSON(people, personJSON)
}
func projectJSON(project *sodav2.Project) any {
	if project == nil {
		return nil
	}
	return map[string]any{"id": project.Id, "slug": project.Slug, "name": project.Name, "unix_user": project.UnixUser, "profile": profileName(project.Profile), "source": projectSourceJSON(project.Source)}
}
func projectsJSON(projects []*sodav2.Project) []any {
	return mapJSON(projects, projectJSON)
}
func projectSourceJSON(source *sodav2.ProjectSource) any {
	if source == nil || source.GetEmpty() != nil {
		return map[string]any{"kind": "empty"}
	}
	if git := source.GetGit(); git != nil {
		return map[string]any{"kind": "git", "remote_url": git.RemoteUrl}
	}
	return nil
}
func membershipJSON(membership *sodav2.Membership) any {
	if membership == nil {
		return nil
	}
	return map[string]any{"project_id": membership.ProjectId, "person_id": membership.PersonId}
}
func collaboratorJSON(collaborator *sodav2.Collaborator) any {
	if collaborator == nil {
		return nil
	}
	return map[string]any{"person": personJSON(collaborator.Person), "membership": membershipJSON(collaborator.Membership), "workspaces": worktreesJSON(collaborator.Worktrees)}
}
func collaboratorsJSON(collaborators []*sodav2.Collaborator) []any {
	return mapJSON(collaborators, collaboratorJSON)
}
func worktreeJSON(worktree *sodav2.Worktree) any {
	if worktree == nil {
		return nil
	}
	return map[string]any{"id": worktree.Id, "project_id": worktree.ProjectId, "person_id": worktree.PersonId, "name": worktree.Name, "branch": worktree.Branch, "path": worktree.Path}
}
func worktreesJSON(worktrees []*sodav2.Worktree) []any {
	return mapJSON(worktrees, worktreeJSON)
}
func jobJSON(job *sodav2.ProvisioningJob) any {
	if job == nil {
		return nil
	}
	var jobError any
	if job.Error != nil {
		jobError = job.GetError()
	}
	return map[string]any{"id": job.Id, "project_id": job.ProjectId, "state": jobStateName(job.State), "error": jobError}
}
func jobsJSON(jobs []*sodav2.ProvisioningJob) []any {
	return mapJSON(jobs, jobJSON)
}
func deployKeyJSON(key *sodav2.DeployKey) any {
	if key == nil {
		return nil
	}
	return map[string]any{"project_id": key.ProjectId, "public_key": key.PublicKey}
}
func toolchainJSON(toolchain *sodav2.ToolchainInstallation) any {
	if toolchain == nil {
		return nil
	}
	return map[string]any{"id": toolchain.Id, "profile": profileName(toolchain.Profile), "version": toolchain.Version, "path": toolchain.Path, "checksum": toolchain.Checksum, "state": jobStateName(toolchain.State)}
}
func osDeploymentJSON(deployment *sodav2.OSDeployment) any {
	if deployment == nil {
		return nil
	}
	return map[string]any{"image_reference": deployment.ImageReference, "version": deployment.Version, "digest": deployment.Digest, "architecture": deployment.Architecture, "signature": deployment.Signature, "incompatible": deployment.Incompatible, "download_only": deployment.DownloadOnly}
}
func osUpdateStatusJSON(value *sodav2.OSUpdateStatus) any {
	if value == nil {
		return nil
	}
	return map[string]any{"booted": osDeploymentJSON(value.Booted), "staged": osDeploymentJSON(value.Staged), "read_only": value.ReadOnly}
}
func osReleaseJSON(value *sodav2.OSRelease) any {
	if value == nil {
		return nil
	}
	return map[string]any{"image_reference": value.ImageReference, "version": value.Version, "source_revision": value.SourceRevision, "fedora_base_reference": value.FedoraBaseReference, "digest": value.Digest, "state_schema": value.StateSchema, "available": value.Available}
}
func roleName(role sodav2.Role) string {
	return strings.ToLower(strings.TrimPrefix(role.String(), "ROLE_"))
}
func profileName(profile sodav2.ToolchainProfile) string {
	return strings.ToLower(strings.TrimPrefix(profile.String(), "TOOLCHAIN_PROFILE_"))
}
func jobStateName(state sodav2.JobState) string {
	return strings.ToLower(strings.TrimPrefix(state.String(), "JOB_STATE_"))
}

func mapJSON[T any](values []T, convert func(T) any) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = convert(value)
	}
	return result
}
