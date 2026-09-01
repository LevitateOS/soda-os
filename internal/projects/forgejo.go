package projects

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

type CreatedRepository struct {
	CanonicalURL string
}

type ForgejoCreateRequest struct {
	BaseURL  string
	Username string
	Password string
	ID       string
}

type forgejoRepositoryResponse struct {
	Name     string `json:"name"`
	CloneURL string `json:"clone_url"`
	Empty    *bool  `json:"empty"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// ForgejoClient uses Forgejo's initiating-user endpoint. It has no Soda-global
// token and does not retain the supplied password.
type ForgejoClient struct{}

func (ForgejoClient) Create(ctx context.Context, creation ForgejoCreateRequest) (CreatedRepository, error) {
	if err := validateForgejoCreation(creation); err != nil {
		return CreatedRepository{}, err
	}
	request, err := newForgejoRepositoryRequest(ctx, creation)
	if err != nil {
		return CreatedRepository{}, err
	}
	httpClient, err := directForgejoHTTPClient()
	if err != nil {
		return CreatedRepository{}, err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return CreatedRepository{}, fmt.Errorf("create Forgejo repository: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return CreatedRepository{}, forgejoRejection(response)
	}
	return decodeCreatedForgejoRepository(response.Body, creation)
}

func validateForgejoCreation(creation ForgejoCreateRequest) error {
	if !projectIDPattern.MatchString(creation.Username) {
		return errors.New("current username is not supported by Forgejo")
	}
	if creation.Password == "" || strings.ContainsAny(creation.Password, "\x00\r\n") {
		return errors.New("Forgejo password is required")
	}
	if !projectIDPattern.MatchString(creation.ID) {
		return errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	return nil
}

func newForgejoRepositoryRequest(ctx context.Context, creation ForgejoCreateRequest) (*http.Request, error) {
	base, err := url.Parse(creation.BaseURL)
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("Forgejo URL is invalid")
	}
	base.Path = strings.TrimSuffix(base.Path, "/") + "/api/v1/user/repos"
	payload, err := json.Marshal(struct {
		Name     string `json:"name"`
		AutoInit bool   `json:"auto_init"`
	}{Name: creation.ID, AutoInit: false})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(creation.Username, creation.Password)
	return request, nil
}

func directForgejoHTTPClient() (*http.Client, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("Forgejo HTTP transport is unavailable")
	}
	directTransport := transport.Clone()
	directTransport.Proxy = nil
	return &http.Client{
		Transport:     directTransport,
		Timeout:       30 * time.Second,
		CheckRedirect: rejectForgejoRedirect,
	}, nil
}

func rejectForgejoRedirect(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

func forgejoRejection(response *http.Response) error {
	diagnostic := forgejoDiagnostic(response.Body)
	if diagnostic == "" {
		return fmt.Errorf("Forgejo rejected repository creation with status %d", response.StatusCode)
	}
	return fmt.Errorf("Forgejo rejected repository creation with status %d: %s", response.StatusCode, diagnostic)
}

func decodeCreatedForgejoRepository(reader io.Reader, creation ForgejoCreateRequest) (CreatedRepository, error) {
	var created forgejoRepositoryResponse
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	if err := decoder.Decode(&created); err != nil {
		return CreatedRepository{}, fmt.Errorf("decode Forgejo repository: %w", err)
	}
	if created.Name != creation.ID || created.Owner.Login != creation.Username {
		return CreatedRepository{}, errors.New("Forgejo returned a repository with unexpected ownership")
	}
	if created.Empty == nil || !*created.Empty {
		return CreatedRepository{}, errors.New("Forgejo did not confirm that the repository is empty")
	}
	if err := ValidateCanonicalURL(created.CloneURL); err != nil {
		return CreatedRepository{}, fmt.Errorf("Forgejo returned an unsafe clone URL: %w", err)
	}
	return CreatedRepository{CanonicalURL: created.CloneURL}, nil
}

func forgejoDiagnostic(reader io.Reader) string {
	contents, err := io.ReadAll(io.LimitReader(reader, 8192))
	if err != nil {
		return ""
	}
	var response struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(contents, &response) == nil && strings.TrimSpace(response.Message) != "" {
		return strings.TrimSpace(response.Message)
	}
	text := strings.TrimSpace(string(contents))
	if strings.IndexFunc(text, unicode.IsControl) >= 0 {
		return ""
	}
	return text
}
