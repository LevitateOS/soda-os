package soda

import (
	"context"
	"fmt"
	"strings"

	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
)

type Person struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        Role   `json:"role"`
}
type SSHDeviceKey struct {
	ID               string `json:"id"`
	PersonID         string `json:"person_id"`
	Label            string `json:"label"`
	Type             string `json:"type"`
	PublicKey        string `json:"public_key"`
	Fingerprint      string `json:"fingerprint"`
	IdentityFileHint string `json:"identity_file_hint"`
	CreatedAt        uint64 `json:"created_at"`
}
type ProjectSource struct {
	Kind      string `json:"kind"`
	RemoteURL string `json:"remote_url,omitempty"`
}
type Project struct {
	ID       string        `json:"id"`
	Slug     string        `json:"slug"`
	Name     string        `json:"name"`
	UnixUser string        `json:"unix_user"`
	Profile  string        `json:"profile"`
	Source   ProjectSource `json:"source"`
}
type Worktree struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	PersonID  string `json:"person_id"`
	Name      string `json:"name"`
	Branch    string `json:"branch"`
	Path      string `json:"path"`
}
type ProvisioningJob struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"project_id"`
	State     string  `json:"state"`
	Error     *string `json:"error"`
}
type ToolchainInstallation struct {
	ID       string `json:"id"`
	Profile  string `json:"profile"`
	Version  string `json:"version"`
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
	State    string `json:"state"`
}
type DeployKey struct {
	ProjectID string `json:"project_id"`
	PublicKey string `json:"public_key"`
}
type RuntimeState string
type ServiceStatus struct {
	Name  string       `json:"name"`
	State RuntimeState `json:"state"`
}
type NetworkInterface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
}
type FilesystemStatus struct {
	Path           string `json:"path"`
	TotalBytes     uint64 `json:"total_bytes"`
	AvailableBytes uint64 `json:"available_bytes"`
}
type HostStatus struct {
	SampledAt            uint64             `json:"sampled_at"`
	Overall              RuntimeState       `json:"overall"`
	Services             []ServiceStatus    `json:"services"`
	SSHFirewallReady     bool               `json:"ssh_firewall_ready"`
	CockpitFirewallReady bool               `json:"cockpit_firewall_ready"`
	Interfaces           []NetworkInterface `json:"interfaces"`
	CPUPercent           *float64           `json:"cpu_percent"`
	LoadAverage          [3]float64         `json:"load_average"`
	UptimeSeconds        uint64             `json:"uptime_seconds"`
	MemoryTotalBytes     uint64             `json:"memory_total_bytes"`
	MemoryAvailableBytes uint64             `json:"memory_available_bytes"`
	Filesystems          []FilesystemStatus `json:"filesystems"`
}
type CreatePersonRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        Role   `json:"role"`
	Password    string `json:"password"`
}
type CreateProjectRequest struct {
	Slug             string        `json:"slug"`
	Name             string        `json:"name"`
	Profile          string        `json:"profile"`
	Source           ProjectSource `json:"source"`
	InitialPersonIDs []string      `json:"initial_person_ids"`
}

type API interface {
	People(context.Context) ([]Person, error)
	Projects(context.Context) ([]Project, error)
	ProjectsForPerson(context.Context, string) ([]Project, error)
	CreatePerson(context.Context, CreatePersonRequest) (Person, error)
	SSHDeviceKeys(context.Context, string) ([]SSHDeviceKey, error)
	CreateSSHDeviceKey(context.Context, string, string, string, string) (SSHDeviceKey, error)
	RevokeSSHDeviceKey(context.Context, string, string) (SSHDeviceKey, error)
	CreateProject(context.Context, CreateProjectRequest) (Project, error)
	Members(context.Context, string) ([]Person, error)
	AddCollaborator(context.Context, string, string) (Worktree, error)
	Worktrees(context.Context, string) ([]Worktree, error)
	Jobs(context.Context, string) ([]ProvisioningJob, error)
	RetryProvisioning(context.Context, string) (ProvisioningJob, error)
	Toolchain(context.Context, string) (*ToolchainInstallation, error)
	DeployKey(context.Context, string) (DeployKey, error)
	HostStatus(context.Context) (HostStatus, error)
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
func (c *Client) CreatePerson(ctx context.Context, request CreatePersonRequest) (Person, error) {
	response, err := c.service.CreatePerson(ctx, &sodav2.CreatePersonRequest{Username: request.Username, DisplayName: request.DisplayName, Email: request.Email, Role: roleToProto(request.Role), Password: request.Password})
	if err != nil {
		return Person{}, rpcError(err)
	}
	return person(response.GetPerson()), nil
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
func (c *Client) CreateSSHDeviceKey(ctx context.Context, personID, label, publicKey, identityFileHint string) (SSHDeviceKey, error) {
	response, err := c.service.CreateSshDeviceKey(ctx, &sodav2.CreateSshDeviceKeyRequest{PersonId: personID, Label: label, PublicKey: publicKey, IdentityFileHint: identityFileHint})
	if err != nil {
		return SSHDeviceKey{}, rpcError(err)
	}
	return sshDeviceKey(response.GetKey()), nil
}
func (c *Client) RevokeSSHDeviceKey(ctx context.Context, personID, keyID string) (SSHDeviceKey, error) {
	response, err := c.service.RevokeSshDeviceKey(ctx, &sodav2.RevokeSshDeviceKeyRequest{PersonId: personID, KeyId: keyID})
	if err != nil {
		return SSHDeviceKey{}, rpcError(err)
	}
	return sshDeviceKey(response.GetKey()), nil
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
func (c *Client) AddCollaborator(ctx context.Context, projectID, personID string) (Worktree, error) {
	response, err := c.service.AddCollaborator(ctx, &sodav2.AddCollaboratorRequest{ProjectId: projectID, PersonId: personID})
	if err != nil {
		return Worktree{}, rpcError(err)
	}
	return worktree(response.GetWorktree()), nil
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
func (c *Client) RetryProvisioning(ctx context.Context, projectID string) (ProvisioningJob, error) {
	response, err := c.service.StartProvisioning(ctx, &sodav2.StartProvisioningRequest{ProjectId: projectID})
	if err != nil {
		return ProvisioningJob{}, rpcError(err)
	}
	return job(response.GetJob()), nil
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
func (c *Client) DeployKey(ctx context.Context, projectID string) (DeployKey, error) {
	response, err := c.service.GetDeployKey(ctx, &sodav2.GetDeployKeyRequest{ProjectId: projectID})
	if err != nil {
		return DeployKey{}, rpcError(err)
	}
	key := response.GetDeployKey()
	return DeployKey{ProjectID: key.GetProjectId(), PublicKey: key.GetPublicKey()}, nil
}
func (c *Client) HostStatus(ctx context.Context) (HostStatus, error) {
	response, err := c.service.GetHostStatus(ctx, &sodav2.GetHostStatusRequest{})
	if err != nil {
		return HostStatus{}, rpcError(err)
	}
	return hostStatus(response.GetHost()), nil
}

type RPCError struct {
	Code    codes.Code
	Message string
}

func (e RPCError) Error() string { return e.Message }
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
	return RPCError{Code: result.Code(), Message: message}
}

func person(value *sodav2.Person) Person {
	return Person{ID: value.GetId(), Username: value.GetUsername(), DisplayName: value.GetDisplayName(), Email: value.GetEmail(), Role: roleFromProto(value.GetRole())}
}
func sshDeviceKey(value *sodav2.SshDeviceKey) SSHDeviceKey {
	keyType := "unknown"
	if fields := strings.Fields(value.GetPublicKey()); len(fields) != 0 {
		keyType = fields[0]
	}
	result := SSHDeviceKey{ID: value.GetId(), PersonID: value.GetPersonId(), Label: value.GetLabel(), Type: keyType, PublicKey: value.GetPublicKey(), Fingerprint: value.GetFingerprint(), IdentityFileHint: value.GetIdentityFileHint()}
	if timestamp := value.GetCreatedAt(); timestamp != nil {
		result.CreatedAt = uint64(timestamp.AsTime().Unix())
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
func hostStatus(value *sodav2.HostStatus) HostStatus {
	result := HostStatus{Overall: runtimeState(value.GetOverall()), SSHFirewallReady: value.GetSshFirewallReady(), CockpitFirewallReady: value.GetCockpitFirewallReady(), UptimeSeconds: value.GetUptimeSeconds(), MemoryTotalBytes: value.GetMemoryTotalBytes(), MemoryAvailableBytes: value.GetMemoryAvailableBytes()}
	if timestamp := value.GetSampledAt(); timestamp != nil {
		result.SampledAt = uint64(timestamp.AsTime().Unix())
	}
	if value.CpuPercent != nil {
		cpu := value.GetCpuPercent()
		result.CPUPercent = &cpu
	}
	if load := value.GetLoadAverage(); load != nil {
		result.LoadAverage = [3]float64{load.GetOneMinute(), load.GetFiveMinutes(), load.GetFifteenMinutes()}
	}
	for _, item := range value.GetServices() {
		result.Services = append(result.Services, ServiceStatus{Name: item.GetName(), State: runtimeState(item.GetState())})
	}
	for _, item := range value.GetInterfaces() {
		result.Interfaces = append(result.Interfaces, NetworkInterface{Name: item.GetName(), Addresses: item.GetAddresses()})
	}
	for _, item := range value.GetFilesystems() {
		result.Filesystems = append(result.Filesystems, FilesystemStatus{Path: item.GetPath(), TotalBytes: item.GetTotalBytes(), AvailableBytes: item.GetAvailableBytes()})
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
func profileFromProto(value sodav2.ToolchainProfile) string {
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
	if value.Kind == "git" {
		return &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Git{Git: &sodav2.GitProjectSource{RemoteUrl: value.RemoteURL}}}
	}
	return &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}
}
func sourceFromProto(value *sodav2.ProjectSource) ProjectSource {
	if git := value.GetGit(); git != nil {
		return ProjectSource{Kind: "git", RemoteURL: git.GetRemoteUrl()}
	}
	return ProjectSource{Kind: "empty"}
}
func jobState(value sodav2.JobState) string {
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
