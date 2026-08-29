package builtingit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
	"github.com/stretchr/testify/require"
)

type apiFixture struct {
	t                *testing.T
	userPayload      map[string]any
	collaborator     bool
	deletedBootstrap bool
}

func (f *apiFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requireToken(r, "test-token")
	switch r.Method + " " + r.URL.Path {
	case "POST /api/v1/admin/users":
		_ = json.NewDecoder(r.Body).Decode(&f.userPayload)
		writeJSON(w, http.StatusCreated, map[string]any{"id": 12})
	case "POST /api/v1/admin/users/alice/keys":
		writeJSON(w, http.StatusCreated, map[string]any{"id": 23})
	case "GET /api/v1/orgs/soda":
		writeJSON(w, http.StatusOK, map[string]any{"id": 1})
	case "POST /api/v1/orgs/soda/repos":
		writeJSON(w, http.StatusCreated, map[string]any{"id": 34, "html_url": "http://soda/repo", "ssh_url": "git@soda:soda/demo.git"})
	case "POST /api/v1/repos/soda/demo/keys":
		writeJSON(w, http.StatusCreated, map[string]any{"id": 45})
	case "PUT /api/v1/repos/soda/demo/collaborators/alice":
		f.collaborator = true
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
	}
}

func (f *apiFixture) requireToken(r *http.Request, token string) {
	require.Equal(f.t, "token "+token, r.Header.Get("Authorization"))
}

type bootstrapFixture struct {
	t       *testing.T
	deleted bool
}

func (f *bootstrapFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method + " " + r.URL.Path {
	case "POST /api/v1/admin/users":
		require.Equal(f.t, "token bootstrap-token", r.Header.Get("Authorization"))
		writeJSON(w, http.StatusCreated, map[string]any{"id": 7})
	case "PATCH /api/v1/admin/users/alice":
		w.WriteHeader(http.StatusNoContent)
	case "DELETE /api/v1/admin/users/soda-bootstrap":
		require.Equal(f.t, "token automation-token", r.Header.Get("Authorization"))
		f.deleted = true
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
	}
}

func bootstrapRunner(t *testing.T) func(context.Context, string, ...string) (string, error) {
	return func(_ context.Context, _ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "admin user create"):
			return "New user 'soda-bootstrap' has been successfully created!\nAccess token was successfully created... bootstrap-token\n", nil
		case strings.Contains(joined, "generate-access-token"):
			return "automation-token\n", nil
		default:
			t.Fatalf("unexpected command %q", joined)
			return "", nil
		}
	}
}

func TestClientUsesPAMUsersAndCreatesRepositoryWithSodaAccess(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture := &apiFixture{t: t}
	server := httptest.NewServer(fixture)
	defer server.Close()
	client := New()
	client.BaseURL, client.TokenPath, client.HTTP = server.URL, tokenPath, server.Client()
	person := domain.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Email: "alice@example.test"}
	user, err := client.EnsurePerson(context.Background(), person, PersonMember)
	require.NoError(t, err)
	require.Equal(t, int64(12), user.ID)
	require.Equal(t, float64(pamSourceID), fixture.userPayload["source_id"])
	require.Equal(t, "alice", fixture.userPayload["login_name"])
	require.Equal(t, "", fixture.userPayload["password"])
	key, err := client.EnsureKey(context.Background(), person, domain.SSHDeviceKey{ID: "key-1", PublicKey: "ssh-ed25519 AAAA alice"})
	require.NoError(t, err)
	require.Equal(t, int64(23), key.ID)
	repository, err := client.EnsureRepository(context.Background(), domain.Project{ID: "project-1", Slug: "demo", Name: "Demo"}, []domain.Person{person}, "ssh-ed25519 AAAA deploy")
	require.NoError(t, err)
	require.Equal(t, int64(34), repository.ID)
	require.Equal(t, int64(45), repository.DeployKeyID)
	require.True(t, fixture.collaborator)
}

func TestFirstSodaPersonReplacesTemporaryBootstrapAccount(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "token")
	fixture := &bootstrapFixture{t: t}
	server := httptest.NewServer(fixture)
	defer server.Close()
	client := New()
	client.BaseURL, client.TokenPath, client.HTTP = server.URL, tokenPath, server.Client()
	client.run = bootstrapRunner(t)
	user, err := client.EnsurePerson(context.Background(), domain.Person{Username: "alice", DisplayName: "Alice", Email: "alice@example.test"}, PersonAdministrator)
	require.NoError(t, err)
	require.Equal(t, int64(7), user.ID)
	require.True(t, fixture.deleted)
	contents, err := os.ReadFile(tokenPath)
	require.NoError(t, err)
	require.Equal(t, "automation-token\n", string(contents))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
