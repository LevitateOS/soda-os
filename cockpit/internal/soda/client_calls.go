package soda

import (
	"context"
	"fmt"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (c *Client) People(ctx context.Context) ([]Person, error) {
	response, err := c.service.ListPeople(ctx, &sodav2.ListPeopleRequest{})
	if err != nil {
		return nil, rpcError(err)
	}
	return people(response.GetPeople()), nil
}
func (c *Client) Projects(ctx context.Context) ([]Project, error) {
	response, err := c.service.ListProjects(ctx, &sodav2.ListProjectsRequest{})
	if err != nil {
		return nil, rpcError(err)
	}
	return projects(response.GetProjects()), nil
}
func (c *Client) ProjectsForPerson(ctx context.Context, personID string) ([]Project, error) {
	response, err := c.service.ListProjectsForPerson(ctx, &sodav2.ListProjectsForPersonRequest{PersonId: personID})
	if err != nil {
		return nil, rpcError(err)
	}
	return projects(response.GetProjects()), nil
}
func (c *Client) CreatePerson(ctx context.Context, request CreatePersonRequest) error {
	_, err := c.service.CreatePerson(ctx, &sodav2.CreatePersonRequest{Username: request.Username, DisplayName: request.DisplayName, Email: request.Email, Role: roleToProto(request.Role), Password: request.Password})
	return rpcError(err)
}
func (c *Client) SSHDeviceKeys(ctx context.Context, personID string) ([]SSHDeviceKey, error) {
	response, err := c.service.ListSshDeviceKeys(ctx, &sodav2.ListSshDeviceKeysRequest{PersonId: personID})
	if err != nil {
		return nil, rpcError(err)
	}
	items := make([]SSHDeviceKey, 0, len(response.GetKeys()))
	for _, item := range response.GetKeys() {
		items = append(items, sshDeviceKey(item))
	}
	return items, nil
}
func (c *Client) CreateSSHDeviceKey(ctx context.Context, personID, label, publicKey, identityFileHint string) error {
	_, err := c.service.CreateSshDeviceKey(ctx, &sodav2.CreateSshDeviceKeyRequest{PersonId: personID, Label: label, PublicKey: publicKey, IdentityFileHint: identityFileHint})
	return rpcError(err)
}
func (c *Client) RevokeSSHDeviceKey(ctx context.Context, personID, keyID string) error {
	_, err := c.service.RevokeSshDeviceKey(ctx, &sodav2.RevokeSshDeviceKeyRequest{PersonId: personID, KeyId: keyID})
	return rpcError(err)
}
func (c *Client) CreateProject(ctx context.Context, request CreateProjectRequest) (Project, error) {
	response, err := c.service.CreateProject(ctx, &sodav2.CreateProjectRequest{Slug: request.Slug, Name: request.Name, Profile: profileToProto(request.Profile), Source: sourceToProto(request.Source), InitialPersonIds: request.InitialPersonIDs})
	if err != nil {
		return Project{}, rpcError(err)
	}
	return project(response.GetProject()), nil
}
func (c *Client) Members(ctx context.Context, projectID string) ([]Person, error) {
	response, err := c.service.ListCollaborators(ctx, &sodav2.ListCollaboratorsRequest{ProjectId: projectID})
	if err != nil {
		return nil, rpcError(err)
	}
	items := make([]Person, 0, len(response.GetCollaborators()))
	for _, collaborator := range response.GetCollaborators() {
		items = append(items, person(collaborator.GetPerson()))
	}
	return items, nil
}
func (c *Client) AddCollaborator(ctx context.Context, command AddCollaboratorCommand) error {
	_, err := c.service.AddCollaborator(ctx, &sodav2.AddCollaboratorRequest{ProjectId: command.ProjectID, PersonId: command.PersonID})
	return rpcError(err)
}
func (c *Client) Worktrees(ctx context.Context, projectID string) ([]Worktree, error) {
	response, err := c.service.ListWorktrees(ctx, &sodav2.ListWorktreesRequest{ProjectId: projectID})
	if err != nil {
		return nil, rpcError(err)
	}
	items := make([]Worktree, 0, len(response.GetWorktrees()))
	for _, item := range response.GetWorktrees() {
		items = append(items, worktree(item))
	}
	return items, nil
}
func (c *Client) Jobs(ctx context.Context, projectID string) ([]ProvisioningJob, error) {
	response, err := c.service.ListProvisioningJobs(ctx, &sodav2.ListProvisioningJobsRequest{ProjectId: projectID})
	if err != nil {
		return nil, rpcError(err)
	}
	items := make([]ProvisioningJob, 0, len(response.GetJobs()))
	for _, item := range response.GetJobs() {
		items = append(items, job(item))
	}
	return items, nil
}
func (c *Client) RetryProvisioning(ctx context.Context, projectID string) error {
	_, err := c.service.StartProvisioning(ctx, &sodav2.StartProvisioningRequest{ProjectId: projectID})
	return rpcError(err)
}
func (c *Client) Toolchain(ctx context.Context, projectID string) (*ToolchainInstallation, error) {
	response, err := c.service.GetProjectToolchain(ctx, &sodav2.GetProjectToolchainRequest{ProjectId: projectID})
	if status.Code(err) == codes.NotFound {
		return nil, nil
	}
	if err != nil {
		return nil, rpcError(err)
	}
	installation := toolchain(response.GetInstallation())
	return &installation, nil
}
func (c *Client) DeployKey(ctx context.Context, projectID string) (string, error) {
	response, err := c.service.GetDeployKey(ctx, &sodav2.GetDeployKeyRequest{ProjectId: projectID})
	if err != nil {
		return "", rpcError(err)
	}
	return response.GetDeployKey().GetPublicKey(), nil
}
func (c *Client) HostStatus(ctx context.Context) (HostStatus, error) {
	response, err := c.service.GetHostStatus(ctx, &sodav2.GetHostStatusRequest{})
	if err != nil {
		return HostStatus{}, rpcError(err)
	}
	return hostStatus(response.GetHost()), nil
}

func (c *Client) OSUpdateStatus(ctx context.Context) (OSUpdateStatus, error) {
	response, err := c.service.GetOSUpdateStatus(ctx, &sodav2.GetOSUpdateStatusRequest{})
	if err != nil {
		return OSUpdateStatus{}, rpcError(err)
	}
	return osUpdateStatus(response.GetStatus()), nil
}

func (c *Client) CheckOSUpdate(ctx context.Context) (OSRelease, error) {
	response, err := c.service.CheckOSUpdate(ctx, &sodav2.CheckOSUpdateRequest{})
	if err != nil {
		return OSRelease{}, rpcError(err)
	}
	value := response.GetRelease()
	if value == nil {
		return OSRelease{}, fmt.Errorf("Soda service returned no OS release")
	}
	return OSRelease{
		ImageReference: value.GetImageReference(), Version: value.GetVersion(), SourceRevision: value.GetSourceRevision(),
		FedoraBaseReference: value.GetFedoraBaseReference(), Digest: value.GetDigest(), StateSchema: value.GetStateSchema(), Available: value.GetAvailable(),
	}, nil
}

func (c *Client) StageOSUpdate(ctx context.Context, imageReference string) (OSUpdateStatus, error) {
	response, err := c.service.StageOSUpdate(ctx, &sodav2.StageOSUpdateRequest{ImageReference: imageReference})
	if err != nil {
		return OSUpdateStatus{}, rpcError(err)
	}
	return osUpdateStatus(response.GetStatus()), nil
}

func (c *Client) ActivateOSUpdate(ctx context.Context) error {
	_, err := c.service.ActivateOSUpdate(ctx, &sodav2.ActivateOSUpdateRequest{ConfirmReboot: true})
	return rpcError(err)
}
