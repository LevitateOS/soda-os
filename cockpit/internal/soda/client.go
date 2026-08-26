package soda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
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
