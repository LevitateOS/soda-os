package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type forgejoRoundTripFunc func(*http.Request) (*http.Response, error)

func (function forgejoRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func forgejoResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestLoopbackForgejoRegistersInitialAdministratorPublicKey(t *testing.T) {
	key := testAdministratorKey(t)
	var paths []string
	client := loopbackForgejoClient{httpClient: &http.Client{Transport: forgejoRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.Method+" "+request.URL.Path)
		require.Equal(t, "http", request.URL.Scheme)
		require.Equal(t, "127.0.0.1:30000", request.URL.Host)
		username, password, ok := request.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "alice", username)
		require.Equal(t, "secret", password)
		switch request.Method + " " + request.URL.Path {
		case "GET /api/v1/user":
			return forgejoResponse(http.StatusOK, `{"login":"alice"}`), nil
		case "GET /api/v1/user/keys":
			return forgejoResponse(http.StatusOK, `[]`), nil
		case "POST /api/v1/user/keys":
			var payload struct {
				Title string `json:"title"`
				Key   string `json:"key"`
			}
			require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
			require.Equal(t, "Soda OS", payload.Title)
			require.Equal(t, key, payload.Key)
			return forgejoResponse(http.StatusCreated, ""), nil
		default:
			return nil, fmt.Errorf("unexpected Forgejo request %s %s", request.Method, request.URL.Path)
		}
	})}}

	err := client.RegisterPublicKey(context.Background(), forgejoKeyRegistration{
		Username: "alice", Password: "secret", PublicKey: key,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"GET /api/v1/user", "GET /api/v1/user/keys", "POST /api/v1/user/keys"}, paths)
}

func TestLoopbackForgejoDoesNotDuplicateExistingPublicKey(t *testing.T) {
	key := testAdministratorKey(t)
	client := loopbackForgejoClient{httpClient: &http.Client{Transport: forgejoRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v1/user":
			return forgejoResponse(http.StatusOK, `{"login":"alice"}`), nil
		case "/api/v1/user/keys":
			require.Equal(t, http.MethodGet, request.Method)
			return forgejoResponse(http.StatusOK, `[{"key":`+fmt.Sprintf("%q", key)+`}]`), nil
		default:
			return nil, fmt.Errorf("unexpected Forgejo request %s %s", request.Method, request.URL.Path)
		}
	})}}

	err := client.RegisterPublicKey(context.Background(), forgejoKeyRegistration{
		Username: "alice", Password: "secret", PublicKey: key,
	})
	require.NoError(t, err)
}
