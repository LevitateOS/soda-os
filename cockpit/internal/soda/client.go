package soda

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
)

type Person struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Email        string `json:"email"`
	Role         Role   `json:"role"`
	SSHPublicKey string `json:"ssh_public_key"`
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
	SSHObserver          RuntimeState       `json:"ssh_observer"`
	GitObserver          RuntimeState       `json:"git_observer"`
}

type WorktreeStatus struct {
	WorktreeID string  `json:"worktree_id"`
	Branch     string  `json:"branch"`
	Head       string  `json:"head"`
	Upstream   *string `json:"upstream"`
	Ahead      uint64  `json:"ahead"`
	Behind     uint64  `json:"behind"`
	Staged     uint64  `json:"staged"`
	Modified   uint64  `json:"modified"`
	Untracked  uint64  `json:"untracked"`
	Conflicted uint64  `json:"conflicted"`
	State      string  `json:"state"`
	Error      *string `json:"error"`
}

type SSHChannel struct {
	Kind       string `json:"kind"`
	WorktreeID string `json:"worktree_id"`
}

type ActiveSSHConnection struct {
	ID            string       `json:"id"`
	ProjectID     string       `json:"project_id"`
	PersonID      string       `json:"person_id"`
	ConnectedAt   uint64       `json:"connected_at"`
	ClientAddress string       `json:"client_address"`
	ClientPort    uint16       `json:"client_port"`
	ServerAddress string       `json:"server_address"`
	ServerPort    uint16       `json:"server_port"`
	Channels      []SSHChannel `json:"channels"`
}

type Event struct {
	Kind      string  `json:"kind"`
	ProjectID *string `json:"project_id"`
	Sequence  uint64  `json:"sequence"`
}

type CreatePersonRequest struct {
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Email        string `json:"email"`
	Role         Role   `json:"role"`
	SSHPublicKey string `json:"ssh_public_key"`
	Password     string `json:"password"`
}

type CreateProjectRequest struct {
	Slug    string        `json:"slug"`
	Name    string        `json:"name"`
	Profile string        `json:"profile"`
	Source  ProjectSource `json:"source"`
}

type API interface {
	People(context.Context) ([]Person, error)
	Projects(context.Context) ([]Project, error)
	ProjectsForPerson(context.Context, string) ([]Project, error)
	CreatePerson(context.Context, CreatePersonRequest) (Person, error)
	CreateProject(context.Context, CreateProjectRequest) (Project, error)
	AddCollaborator(context.Context, string, string) (Worktree, error)
	CreateWorktree(context.Context, string, string, string, string) (Worktree, error)
	Worktrees(context.Context, string) ([]Worktree, error)
	Jobs(context.Context, string) ([]ProvisioningJob, error)
	RetryProvisioning(context.Context, string) (ProvisioningJob, error)
	Toolchain(context.Context, string) (*ToolchainInstallation, error)
	DeployKey(context.Context, string) (DeployKey, error)
	HostStatus(context.Context) (HostStatus, error)
	WorktreeStatuses(context.Context, string) ([]WorktreeStatus, error)
	ActiveSessions(context.Context) ([]ActiveSSHConnection, error)
	Events(context.Context, string) (<-chan Event, error)
}

type Client struct {
	http *http.Client
}

func NewClient(socketPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{http: &http.Client{Transport: transport}}
}

func (c *Client) People(ctx context.Context) ([]Person, error) {
	return get[[]Person](ctx, c, "/v1/people")
}

func (c *Client) Projects(ctx context.Context) ([]Project, error) {
	return get[[]Project](ctx, c, "/v1/projects")
}

func (c *Client) ProjectsForPerson(ctx context.Context, personID string) ([]Project, error) {
	return get[[]Project](ctx, c, "/v1/people/"+personID+"/projects")
}

func (c *Client) CreatePerson(ctx context.Context, request CreatePersonRequest) (Person, error) {
	return post[Person](ctx, c, "/v1/people", request)
}

func (c *Client) CreateProject(ctx context.Context, request CreateProjectRequest) (Project, error) {
	return post[Project](ctx, c, "/v1/projects", request)
}

func (c *Client) AddCollaborator(ctx context.Context, projectID, personID string) (Worktree, error) {
	return post[Worktree](ctx, c, "/v1/projects/"+projectID+"/collaborators", map[string]string{"person_id": personID})
}

func (c *Client) CreateWorktree(ctx context.Context, projectID, personID, name, baseRef string) (Worktree, error) {
	return post[Worktree](ctx, c, "/v1/projects/"+projectID+"/worktrees", map[string]string{
		"person_id": personID,
		"name":      name,
		"base_ref":  baseRef,
	})
}

func (c *Client) Worktrees(ctx context.Context, projectID string) ([]Worktree, error) {
	return get[[]Worktree](ctx, c, "/v1/projects/"+projectID+"/worktrees")
}

func (c *Client) Jobs(ctx context.Context, projectID string) ([]ProvisioningJob, error) {
	return get[[]ProvisioningJob](ctx, c, "/v1/projects/"+projectID+"/provisioning")
}

func (c *Client) RetryProvisioning(ctx context.Context, projectID string) (ProvisioningJob, error) {
	return post[ProvisioningJob](ctx, c, "/v1/projects/"+projectID+"/provisioning", struct{}{})
}

func (c *Client) Toolchain(ctx context.Context, projectID string) (*ToolchainInstallation, error) {
	installation, err := get[ToolchainInstallation](ctx, c, "/v1/projects/"+projectID+"/toolchain")
	if status, ok := err.(StatusError); ok && status.Code == http.StatusNotFound {
		return nil, nil
	}
	return &installation, err
}

func (c *Client) DeployKey(ctx context.Context, projectID string) (DeployKey, error) {
	return get[DeployKey](ctx, c, "/v1/projects/"+projectID+"/deploy-key")
}

func (c *Client) HostStatus(ctx context.Context) (HostStatus, error) {
	return get[HostStatus](ctx, c, "/v1/host-status")
}

func (c *Client) WorktreeStatuses(ctx context.Context, projectID string) ([]WorktreeStatus, error) {
	return get[[]WorktreeStatus](ctx, c, "/v1/projects/"+projectID+"/worktree-status")
}

func (c *Client) ActiveSessions(ctx context.Context) ([]ActiveSSHConnection, error) {
	return get[[]ActiveSSHConnection](ctx, c, "/v1/ssh-sessions")
}

func (c *Client) Events(ctx context.Context, projectID string) (<-chan Event, error) {
	path := "http://sodad/v1/events"
	if projectID != "" {
		path += "?project_id=" + url.QueryEscape(projectID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		payload, _ := io.ReadAll(response.Body)
		return nil, StatusError{Code: response.StatusCode, Body: strings.TrimSpace(string(payload))}
	}
	events := make(chan Event, 32)
	go func() {
		defer close(events)
		defer response.Body.Close()
		scanner := bufio.NewScanner(response.Body)
		var name string
		var data strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event:"):
				name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				if data.Len() > 0 {
					data.WriteByte('\n')
				}
				data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			case line == "":
				event := Event{Kind: name}
				if data.Len() > 0 && data.String() != "refresh" {
					_ = json.Unmarshal([]byte(data.String()), &event)
				}
				if event.Kind != "" {
					select {
					case events <- event:
					case <-ctx.Done():
						return
					}
				}
				name = ""
				data.Reset()
			}
		}
	}()
	return events, nil
}

type StatusError struct {
	Code int
	Body string
}

func (e StatusError) Error() string {
	return fmt.Sprintf("sodad returned HTTP %d: %s", e.Code, e.Body)
}

func get[T any](ctx context.Context, client *Client, path string) (T, error) {
	return request[T](ctx, client, http.MethodGet, path, nil)
}

func post[T any](ctx context.Context, client *Client, path string, body any) (T, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		var zero T
		return zero, err
	}
	return request[T](ctx, client, http.MethodPost, path, bytes.NewReader(encoded))
}

func request[T any](ctx context.Context, client *Client, method, path string, body io.Reader) (T, error) {
	var result T
	req, err := http.NewRequestWithContext(ctx, method, "http://sodad"+path, body)
	if err != nil {
		return result, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.http.Do(req)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return result, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, StatusError{Code: response.StatusCode, Body: strings.TrimSpace(string(payload))}
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		return result, err
	}
	return result, nil
}
