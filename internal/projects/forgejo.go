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

type ForgejoKeyRequest struct {
	BaseURL   string
	Username  string
	Password  string
	PublicKey string
}

type forgejoUserResponse struct {
	Login string `json:"login"`
}

type forgejoKeyResponse struct {
	Key string `json:"key"`
}

// ForgejoClient is the one-shot Soda Setup adapter for the initial
// administrator. It has no Soda-global token and does not retain the supplied
// password. Projects does not use it.
type ForgejoClient struct{}

// RegisterPublicKey authenticates the initial administrator once through
// Forgejo's native PAM boundary. Forgejo owns the resulting account and
// public-key record; Soda retains neither a token nor an identity mirror.
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
	}{Title: "Soda OS", Key: registration.PublicKey})
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
		return fmt.Errorf("Forgejo rejected the request with status %d", response.StatusCode)
	}
	return fmt.Errorf("Forgejo rejected the request with status %d: %s", response.StatusCode, diagnostic)
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
