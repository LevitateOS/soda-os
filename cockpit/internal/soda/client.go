package soda

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"github.com/LevitateOS/soda-os/internal/osupdate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Role = domain.Role

const (
	RoleAdmin     = domain.RoleAdmin
	RoleDeveloper = domain.RoleDeveloper
)

type Person = domain.Person
type SSHDeviceKey = domain.SSHDeviceKey
type ProjectSource = domain.ProjectSource
type EmptyProjectSource = domain.EmptyProjectSource
type GitProjectSource = domain.GitProjectSource
type Project = domain.Project
type Worktree = domain.Worktree
type ToolchainProfile = domain.ToolchainProfile
type JobState = domain.JobState
type ProvisioningJob = domain.ProvisioningJob
type ToolchainInstallation = domain.ToolchainInstallation
type RuntimeState = domain.RuntimeState
type ServiceStatus = domain.ServiceStatus
type NetworkInterface = domain.NetworkInterface
type FilesystemStatus = domain.FilesystemStatus
type HostHealth = domain.HostHealth
type HostNetwork = domain.HostNetwork
type FirewallStatus = domain.FirewallStatus
type HostResources = domain.HostResources
type HostStatus = domain.HostStatus
type OSDeployment = osupdate.Deployment
type OSUpdateStatus = osupdate.Status
type OSRelease = osupdate.Candidate
type CreatePersonRequest struct {
	Username    string
	DisplayName string
	Email       string
	Role        Role
	Password    string
}
type CreateProjectRequest struct {
	Slug             string
	Name             string
	Profile          string
	Source           ProjectSource
	InitialPersonIDs []string
}

type AddCollaboratorCommand struct {
	ProjectID string
	PersonID  string
}

type Client struct {
	service    sodav2.SodaServiceClient
	connection *grpc.ClientConn
}

// NewClient connects to the private Soda daemon over its Unix-domain gRPC socket.
func NewClient(socketPath string) (*Client, error) {
	connection, err := grpcclient.New(socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial sodad: %w", err)
	}
	return newClient(sodav2.NewSodaServiceClient(connection), connection), nil
}
func newClient(service sodav2.SodaServiceClient, connection *grpc.ClientConn) *Client {
	return &Client{service: service, connection: connection}
}
func (c *Client) Close() error {
	if c.connection == nil {
		return nil
	}
	err := c.connection.Close()
	c.connection = nil
	return err
}

func rpcError(err error) error {
	if err == nil {
		return nil
	}
	result := status.Convert(err)
	message := "Soda request failed."
	switch result.Code() {
	case codes.InvalidArgument, codes.AlreadyExists, codes.FailedPrecondition, codes.NotFound:
		message = result.Message()
	case codes.Unavailable, codes.DeadlineExceeded:
		message = "Soda service unavailable."
	case codes.Internal, codes.Unknown, codes.DataLoss:
		message = "Soda service error."
	case codes.PermissionDenied, codes.Unauthenticated:
		message = "Permission denied."
	case codes.Canceled:
		message = "Request canceled."
	}
	return fmt.Errorf("%s", message)
}
