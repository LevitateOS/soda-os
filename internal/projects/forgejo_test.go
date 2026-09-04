package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForgejoRegistersInitialAdministratorPublicKey(t *testing.T) {
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
