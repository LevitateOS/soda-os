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

type ForgejoKeyRequest struct {
	BaseURL   string
	Username  string
	Password  string
	PublicKey string
	Title     string
}

type forgejoRepositoryResponse struct {
	Name   string `json:"name"`
	SSHURL string `json:"ssh_url"`
	Empty  *bool  `json:"empty"`
	Owner  struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type forgejoUserResponse struct {
	Login string `json:"login"`
}

type forgejoKeyResponse struct {
	Key string `json:"key"`
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

// RegisterPublicKey authenticates the person once through Forgejo's native PAM
// boundary. Forgejo owns the resulting account and public-key record; Soda
// retains neither a token nor an identity mirror.
func (ForgejoClient) RegisterPublicKey(ctx context.Context, registration ForgejoKeyRequest) error {
	if err := validateForgejoKeyRegistration(registration); err != nil {
		return err
	}
	httpClient, err := directForgejoHTTPClient()
	if err != nil {
		return err
	}
	if err = confirmForgejoUser(ctx, httpClient, registration); err != nil {
		return err
	}
	keys, err := listForgejoKeys(ctx, httpClient, registration)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if strings.TrimSpace(key.Key) == registration.PublicKey {
			return nil
		}
	}
	return createForgejoKey(ctx, httpClient, registration)
}

func validateForgejoKeyRegistration(registration ForgejoKeyRequest) error {
	if !projectIDPattern.MatchString(registration.Username) {
		return errors.New("username is not supported by Forgejo")
	}
	if err := validateHumanPassword(registration.Password); err != nil {
		return err
	}
	key, err := CanonicalAuthorizedKey(registration.PublicKey)
	if err != nil || key != registration.PublicKey {
		return errors.New("Forgejo public key is invalid")
	}
	return nil
}

func confirmForgejoUser(ctx context.Context, httpClient *http.Client, registration ForgejoKeyRequest) error {
	request, err := newForgejoUserRequest(ctx, http.MethodGet, registration, "/api/v1/user", nil)
	if err != nil {
		return err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("authenticate Forgejo user: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return forgejoRejection(response)
	}
	var user forgejoUserResponse
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&user); err != nil {
		return fmt.Errorf("decode Forgejo user: %w", err)
	}
	if user.Login != registration.Username {
		return errors.New("Forgejo authenticated an unexpected user")
	}
	return nil
}

func listForgejoKeys(ctx context.Context, httpClient *http.Client, registration ForgejoKeyRequest) ([]forgejoKeyResponse, error) {
	request, err := newForgejoUserRequest(ctx, http.MethodGet, registration, "/api/v1/user/keys", nil)
	if err != nil {
		return nil, err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("list Forgejo SSH keys: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, forgejoRejection(response)
	}
	var keys []forgejoKeyResponse
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&keys); err != nil {
		return nil, fmt.Errorf("decode Forgejo SSH keys: %w", err)
	}
	return keys, nil
}

func createForgejoKey(ctx context.Context, httpClient *http.Client, registration ForgejoKeyRequest) error {
	payload, err := json.Marshal(struct {
		Title string `json:"title"`
		Key   string `json:"key"`
	}{Title: forgejoKeyTitle(registration), Key: registration.PublicKey})
	if err != nil {
		return err
	}
	request, err := newForgejoUserRequest(ctx, http.MethodPost, registration, "/api/v1/user/keys", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("register Forgejo SSH key: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return forgejoRejection(response)
	}
	return nil
}

func forgejoKeyTitle(registration ForgejoKeyRequest) string {
	if registration.Title != "" {
		return registration.Title
	}
	return "Soda OS"
}

func newForgejoUserRequest(ctx context.Context, method string, registration ForgejoKeyRequest, endpoint string, body io.Reader) (*http.Request, error) {
	base, err := url.Parse(registration.BaseURL)
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("Forgejo URL is invalid")
	}
	base.Path = strings.TrimSuffix(base.Path, "/") + endpoint
	request, err := http.NewRequestWithContext(ctx, method, base.String(), body)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(registration.Username, registration.Password)
	return request, nil
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
	if err := ValidateCanonicalURL(created.SSHURL); err != nil {
		return CreatedRepository{}, fmt.Errorf("Forgejo returned an unsafe clone URL: %w", err)
	}
	return CreatedRepository{CanonicalURL: created.SSHURL}, nil
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
