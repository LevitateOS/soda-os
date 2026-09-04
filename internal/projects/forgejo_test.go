package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForgejoCreatesEmptyRepositoryAsInitiatingUser(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/api/v1/user/repos", request.URL.Path)
		username, password, ok := request.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "alice", username)
		require.Equal(t, "secret", password)
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"name":"site","clone_url":"https://forgejo.example.test/alice/site.git","empty":true,"owner":{"login":"alice"}}`))
	}))
	defer server.Close()

	repository, err := createForgejoTestRepository(server.URL)
	require.NoError(t, err)
	require.Equal(t, "https://forgejo.example.test/alice/site.git", repository.CanonicalURL)
	require.Equal(t, false, payload["auto_init"])
	require.NotContains(t, payload, "private", "Forgejo's native default owns visibility")
	require.NotContains(t, payload, "description", "catalog presentation is not copied into Forgejo")
}

func TestForgejoRejectsUnexpectedOwnershipOrNonEmptyRepository(t *testing.T) {
	for name, response := range map[string]string{
		"owner":   `{"name":"site","clone_url":"https://forgejo.test/bob/site.git","empty":true,"owner":{"login":"bob"}}`,
		"content": `{"name":"site","clone_url":"https://forgejo.test/alice/site.git","empty":false,"owner":{"login":"alice"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusCreated)
				_, _ = writer.Write([]byte(response))
			}))
			defer server.Close()
			_, err := createForgejoTestRepository(server.URL)
			require.Error(t, err)
		})
	}
}

func TestForgejoSurfacesBoundedNativeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusConflict)
		_, _ = writer.Write([]byte(`{"message":"repository already exists"}`))
	}))
	defer server.Close()

	_, err := createForgejoTestRepository(server.URL)
	require.ErrorContains(t, err, "status 409: repository already exists")
}

func TestForgejoUsesADirectNonRedirectingTransport(t *testing.T) {
	originalTransport := http.DefaultTransport
	proxyCalled := false
	http.DefaultTransport = &http.Transport{Proxy: func(*http.Request) (*url.URL, error) {
		proxyCalled = true
		return nil, errors.New("ambient proxy must not be consulted")
	}}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"name":"site","clone_url":"https://forgejo.test/alice/site.git","empty":true,"owner":{"login":"alice"}}`))
	}))
	defer server.Close()

	_, err := createForgejoTestRepository(server.URL)
	require.NoError(t, err)
	require.False(t, proxyCalled)

	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, server.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	_, err = createForgejoTestRepository(redirect.URL)
	require.ErrorContains(t, err, "status 307")
}

func TestForgejoRegistersPublicKeyThroughThePersonAccount(t *testing.T) {
	key := strings.TrimSpace(string(testAuthorizedKey(t)))
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.Method+" "+request.URL.Path)
		username, password, ok := request.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "alice", username)
		require.Equal(t, "secret", password)
		switch request.Method + " " + request.URL.Path {
		case "GET /api/v1/user":
			_, _ = writer.Write([]byte(`{"login":"alice"}`))
		case "GET /api/v1/user/keys":
			_, _ = writer.Write([]byte(`[]`))
		case "POST /api/v1/user/keys":
			var payload struct {
				Title string `json:"title"`
				Key   string `json:"key"`
			}
			require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
			require.Equal(t, "Soda OS", payload.Title)
			require.Equal(t, key, payload.Key)
			writer.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected Forgejo request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()

	err := (ForgejoClient{}).RegisterPublicKey(context.Background(), ForgejoKeyRequest{
		BaseURL: server.URL, Username: "alice", Password: "secret", PublicKey: key,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"GET /api/v1/user", "GET /api/v1/user/keys", "POST /api/v1/user/keys"}, paths)
}

func TestForgejoDoesNotDuplicateAnExistingPublicKey(t *testing.T) {
	key := strings.TrimSpace(string(testAuthorizedKey(t)))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/user":
			_, _ = writer.Write([]byte(`{"login":"alice"}`))
		case "/api/v1/user/keys":
			require.Equal(t, http.MethodGet, request.Method)
			_, _ = writer.Write([]byte(`[{"key":` + fmt.Sprintf("%q", key) + `}]`))
		default:
			t.Fatalf("unexpected Forgejo request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()

	err := (ForgejoClient{}).RegisterPublicKey(context.Background(), ForgejoKeyRequest{
		BaseURL: server.URL, Username: "alice", Password: "secret", PublicKey: key,
	})
	require.NoError(t, err)
}

func createForgejoTestRepository(baseURL string) (CreatedRepository, error) {
	return (ForgejoClient{}).Create(context.Background(), ForgejoCreateRequest{
		BaseURL:  baseURL,
		Username: "alice",
		Password: "secret",
		ID:       "site",
	})
}
