package builtingit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

const (
	DefaultURL       = "http://127.0.0.1:30000"
	DefaultTokenPath = "/var/lib/soda/built-in-git-token"
	DefaultConfig    = "/etc/forgejo/app.ini"
	organization     = "soda"
	pamSourceID      = int64(1)
)

type User struct{ ID int64 }

type PersonKind uint8

const (
	PersonMember PersonKind = iota
	PersonAdministrator
)

type Key struct{ ID int64 }

type Repository struct {
	ID          int64
	DeployKeyID int64
	WebURL      string
	SSHURL      string
}

type Client struct {
	BaseURL   string
	TokenPath string
	Binary    string
	Config    string
	RunAs     string
	HTTP      *http.Client
	run       func(context.Context, string, ...string) (string, error)
}

func New() *Client {
	c := &Client{BaseURL: DefaultURL, TokenPath: DefaultTokenPath, Binary: "/usr/bin/forgejo", Config: DefaultConfig, RunAs: "git", HTTP: &http.Client{Timeout: 30 * time.Second}}
	c.run = c.runCommand
	return c
}

func (c *Client) EnsurePerson(ctx context.Context, person domain.Person, kind PersonKind) (User, error) {
	token, err := c.token()
	if errors.Is(err, os.ErrNotExist) {
		if kind != PersonAdministrator {
			return User{}, errors.New("Built-in Git administrator is not initialized")
		}
		return c.bootstrap(ctx, person)
	}
	if err != nil {
		return User{}, err
	}
	return c.ensurePersonWithToken(ctx, token, person, kind)
}

func (c *Client) EnsureKey(ctx context.Context, person domain.Person, key domain.SSHDeviceKey) (Key, error) {
	token, err := c.token()
	if err != nil {
		return Key{}, err
	}
	title := "soda-" + key.ID
	payload := map[string]any{"title": title, "key": key.PublicKey}
	var created struct {
		ID int64 `json:"id"`
	}
	status, err := c.request(ctx, token, apiCall{http.MethodPost, "/api/v1/admin/users/" + url.PathEscape(person.Username) + "/keys", payload}, &created)
	if err == nil && status == http.StatusCreated {
		return Key{ID: created.ID}, nil
	}
	if status != http.StatusUnprocessableEntity && status != http.StatusConflict {
		return Key{}, err
	}
	var existing []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	}
	if _, listErr := c.request(ctx, token, apiCall{Method: http.MethodGet, Path: "/api/v1/users/" + url.PathEscape(person.Username) + "/keys"}, &existing); listErr != nil {
		return Key{}, errors.Join(err, listErr)
	}
	for _, item := range existing {
		if item.Title == title {
			return Key{ID: item.ID}, nil
		}
	}
	return Key{}, err
}

func (c *Client) DeleteKey(ctx context.Context, username string, keyID int64) error {
	token, err := c.token()
	if err != nil {
		return err
	}
	status, err := c.request(ctx, token, apiCall{Method: http.MethodDelete, Path: "/api/v1/admin/users/" + url.PathEscape(username) + "/keys/" + strconv.FormatInt(keyID, 10)}, nil)
	if status == http.StatusNotFound {
		return nil
	}
	return err
}

func (c *Client) EnsureRepository(ctx context.Context, project domain.Project, members []domain.Person, deployKey string) (Repository, error) {
	token, err := c.token()
	if err != nil {
		return Repository{}, err
	}
	if err = c.ensureOrganization(ctx, token); err != nil {
		return Repository{}, err
	}
	repository, err := c.ensureRepositoryRecord(ctx, token, project)
	if err != nil {
		return Repository{}, err
	}
	deploy, err := c.ensureDeployKey(ctx, token, project, deployKey)
	if err != nil {
		return Repository{}, err
	}
	if err = c.ensureCollaborators(ctx, token, project.Slug, members); err != nil {
		return Repository{}, err
	}
	return Repository{ID: repository.ID, DeployKeyID: deploy.ID, WebURL: repository.HTMLURL, SSHURL: repository.SSHURL}, nil
}

type repositoryResponse struct {
	ID      int64  `json:"id"`
	HTMLURL string `json:"html_url"`
	SSHURL  string `json:"ssh_url"`
}

func (c *Client) ensureRepositoryRecord(ctx context.Context, token string, project domain.Project) (repositoryResponse, error) {
	payload := map[string]any{"name": project.Slug, "description": project.Name, "private": false, "auto_init": false, "default_branch": "main"}
	var repository repositoryResponse
	status, createErr := c.request(ctx, token, apiCall{http.MethodPost, "/api/v1/orgs/" + organization + "/repos", payload}, &repository)
	if createErr == nil {
		return repository, nil
	}
	if status != http.StatusConflict && status != http.StatusUnprocessableEntity {
		return repositoryResponse{}, createErr
	}
	_, err := c.request(ctx, token, apiCall{Method: http.MethodGet, Path: "/api/v1/repos/" + organization + "/" + url.PathEscape(project.Slug)}, &repository)
	if err != nil {
		return repositoryResponse{}, errors.Join(createErr, err)
	}
	return repository, nil
}

func (c *Client) ensureCollaborators(ctx context.Context, token, project string, members []domain.Person) error {
	for _, member := range members {
		path := "/api/v1/repos/" + organization + "/" + url.PathEscape(project) + "/collaborators/" + url.PathEscape(member.Username)
		if _, err := c.request(ctx, token, apiCall{http.MethodPut, path, map[string]any{"permission": "write"}}, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) ensureDeployKey(ctx context.Context, token string, project domain.Project, publicKey string) (Key, error) {
	title := "soda-project-" + project.ID
	path := "/api/v1/repos/" + organization + "/" + url.PathEscape(project.Slug) + "/keys"
	var created struct {
		ID int64 `json:"id"`
	}
	status, err := c.request(ctx, token, apiCall{http.MethodPost, path, map[string]any{"title": title, "key": publicKey, "read_only": false}}, &created)
	if err == nil {
		return Key{ID: created.ID}, nil
	}
	if status != http.StatusConflict && status != http.StatusUnprocessableEntity {
		return Key{}, err
	}
	var existing []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	}
	if _, listErr := c.request(ctx, token, apiCall{Method: http.MethodGet, Path: path}, &existing); listErr != nil {
		return Key{}, errors.Join(err, listErr)
	}
	for _, item := range existing {
		if item.Title == title {
			return Key{ID: item.ID}, nil
		}
	}
	return Key{}, err
}

func (c *Client) ensureOrganization(ctx context.Context, token string) error {
	status, err := c.request(ctx, token, apiCall{Method: http.MethodGet, Path: "/api/v1/orgs/" + organization}, nil)
	if err == nil {
		return nil
	}
	if status != http.StatusNotFound {
		return err
	}
	var current struct {
		Username string `json:"login"`
	}
	if _, err = c.request(ctx, token, apiCall{Method: http.MethodGet, Path: "/api/v1/user"}, &current); err != nil {
		return err
	}
	status, err = c.request(ctx, token, apiCall{http.MethodPost, "/api/v1/admin/users/" + url.PathEscape(current.Username) + "/orgs", map[string]any{"username": organization, "full_name": "Soda OS", "visibility": "public"}}, nil)
	if status == http.StatusUnprocessableEntity || status == http.StatusConflict {
		return nil
	}
	return err
}

func (c *Client) bootstrap(ctx context.Context, person domain.Person) (User, error) {
	bootstrapToken, err := c.bootstrapToken(ctx)
	if err != nil {
		return User{}, err
	}
	user, err := c.ensurePersonWithToken(ctx, bootstrapToken, person, PersonAdministrator)
	if err != nil {
		return User{}, err
	}
	output, err := c.command(ctx, "admin", "user", "generate-access-token", "--username", person.Username, "--token-name", "soda-automation", "--scopes", "all", "--raw")
	if err != nil {
		return User{}, err
	}
	token := strings.TrimSpace(output)
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return User{}, errors.New("Built-in Git generated an invalid automation token")
	}
	if err = writeSecret(c.TokenPath, token+"\n"); err != nil {
		return User{}, err
	}
	_, deleteErr := c.request(ctx, token, apiCall{Method: http.MethodDelete, Path: "/api/v1/admin/users/soda-bootstrap?purge=true"}, nil)
	if deleteErr != nil {
		return User{}, fmt.Errorf("remove temporary Built-in Git bootstrap account: %w", deleteErr)
	}
	return user, nil
}

func (c *Client) bootstrapToken(ctx context.Context) (string, error) {
	name := "soda-bootstrap-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	output, err := c.command(ctx, "admin", "user", "create", "--username", "soda-bootstrap", "--email", "soda-bootstrap@localhost", "--random-password", "--random-password-length", "32", "--admin", "--must-change-password=false", "--access-token", "--access-token-name", name, "--access-token-scopes", "all")
	if err != nil {
		output, err = c.command(ctx, "admin", "user", "generate-access-token", "--username", "soda-bootstrap", "--token-name", name, "--scopes", "all", "--raw")
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(output), nil
	}
	marker := "Access token was successfully created... "
	index := strings.LastIndex(output, marker)
	if index < 0 {
		return "", errors.New("Built-in Git bootstrap token was not returned")
	}
	return strings.TrimSpace(output[index+len(marker):]), nil
}

func (c *Client) ensurePersonWithToken(ctx context.Context, token string, person domain.Person, kind PersonKind) (User, error) {
	payload := map[string]any{"source_id": pamSourceID, "login_name": person.Username, "username": person.Username, "full_name": person.DisplayName, "email": person.Email, "password": "", "must_change_password": false, "restricted": false}
	var created struct {
		ID int64 `json:"id"`
	}
	status, err := c.request(ctx, token, apiCall{http.MethodPost, "/api/v1/admin/users", payload}, &created)
	if err != nil {
		if status != http.StatusUnprocessableEntity && status != http.StatusConflict {
			return User{}, err
		}
		if _, err = c.request(ctx, token, apiCall{Method: http.MethodGet, Path: "/api/v1/users/" + url.PathEscape(person.Username)}, &created); err != nil {
			return User{}, err
		}
	}
	if kind == PersonAdministrator {
		admin := true
		if _, err = c.request(ctx, token, apiCall{http.MethodPatch, "/api/v1/admin/users/" + url.PathEscape(person.Username), map[string]any{"admin": admin}}, nil); err != nil {
			return User{}, err
		}
	}
	return User{ID: created.ID}, nil
}

func (c *Client) token() (string, error) {
	contents, err := os.ReadFile(c.TokenPath)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(contents))
	if token == "" {
		return "", errors.New("Built-in Git automation token is empty")
	}
	return token, nil
}

func (c *Client) command(ctx context.Context, args ...string) (string, error) {
	command := append([]string{"--config", c.Config}, args...)
	return c.run(ctx, c.Binary, command...)
}

func (c *Client) runCommand(ctx context.Context, binary string, args ...string) (string, error) {
	commandArgs := append([]string{"--user", c.RunAs, "--", binary}, args...)
	command := exec.CommandContext(ctx, "runuser", commandArgs...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("Built-in Git command failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

type apiCall struct {
	Method  string
	Path    string
	Payload any
}

func (c *Client) request(ctx context.Context, token string, call apiCall, destination any) (int, error) {
	var body io.Reader
	if call.Payload != nil {
		contents, err := json.Marshal(call.Payload)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(contents)
	}
	request, err := http.NewRequestWithContext(ctx, call.Method, strings.TrimRight(c.BaseURL, "/")+call.Path, body)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "token "+token)
	if call.Payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return 0, fmt.Errorf("Built-in Git API: %w", err)
	}
	defer response.Body.Close()
	return decodeResponse(response, call, destination)
}

func decodeResponse(response *http.Response, call apiCall, destination any) (int, error) {
	contents, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return response.StatusCode, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, fmt.Errorf("Built-in Git API %s %s returned %s: %s", call.Method, call.Path, response.Status, strings.TrimSpace(string(contents)))
	}
	if destination != nil && len(contents) != 0 {
		if err = json.Unmarshal(contents, destination); err != nil {
			return response.StatusCode, fmt.Errorf("decode Built-in Git API response: %w", err)
		}
	}
	return response.StatusCode, nil
}

func writeSecret(path, contents string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".built-in-git-token-")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.WriteString(contents)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
