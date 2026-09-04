package setup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
)

type forgejoKeyRegistration struct {
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

// loopbackForgejoClient is the one-shot Soda Setup adapter for the initial
// administrator. It retains neither a credential nor an identity mirror.
type loopbackForgejoClient struct {
	httpClient *http.Client
}

func (client loopbackForgejoClient) RegisterPublicKey(ctx context.Context, registration forgejoKeyRegistration) error {
	if err := validateForgejoKeyRegistration(registration); err != nil {
		return err
	}
	httpClient, err := client.directHTTPClient()
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

func (client loopbackForgejoClient) directHTTPClient() (*http.Client, error) {
	if client.httpClient != nil {
		return client.httpClient, nil
	}
	return directLocalHTTPClient(30 * time.Second)
}

func validateForgejoKeyRegistration(registration forgejoKeyRegistration) error {
	if err := validateAdministratorUsername(registration.Username); err != nil {
		return errors.New("username is not supported by Forgejo")
	}
	if err := validateAdministratorPassword(registration.Password); err != nil {
		return err
	}
	key, err := linuxhost.CanonicalAuthorizedKey(registration.PublicKey)
	if err != nil || key != registration.PublicKey {
		return errors.New("Forgejo public key is invalid")
	}
	return nil
}

func confirmForgejoUser(ctx context.Context, client *http.Client, registration forgejoKeyRegistration) error {
	request, err := newForgejoUserRequest(ctx, http.MethodGet, registration, "/api/v1/user", nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
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

func listForgejoKeys(ctx context.Context, client *http.Client, registration forgejoKeyRegistration) ([]forgejoKeyResponse, error) {
	request, err := newForgejoUserRequest(ctx, http.MethodGet, registration, "/api/v1/user/keys", nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
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

func createForgejoKey(ctx context.Context, client *http.Client, registration forgejoKeyRegistration) error {
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
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("register Forgejo SSH key: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return forgejoRejection(response)
	}
	return nil
}

func newForgejoUserRequest(ctx context.Context, method string, registration forgejoKeyRegistration, endpoint string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, forgejoBaseURL+endpoint, body)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(registration.Username, registration.Password)
	return request, nil
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
