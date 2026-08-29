package builtingit

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestRealForgejoBuiltInGitHappyPath(t *testing.T) {
	binary := os.Getenv("SODA_FORGEJO_BINARY")
	if binary == "" {
		t.Skip("set SODA_FORGEJO_BINARY to run the real Built-in Git integration")
	}
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".ssh"), 0o700))
	port := freePort(t)
	configuration := filepath.Join(root, "app.ini")
	require.NoError(t, os.WriteFile(configuration, []byte(testConfiguration(root, port)), 0o600))
	runForgejo(t, binary, configuration, "migrate")
	runForgejo(t, binary, configuration, "admin", "auth", "add-pam", "--name", "Soda OS", "--service-name", "login", "--active")

	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, binary, "--config", configuration, "web")
	logPath := filepath.Join(root, "forgejo.log")
	log, err := os.Create(logPath)
	require.NoError(t, err)
	command.Stdout, command.Stderr = log, log
	require.NoError(t, command.Start())
	t.Cleanup(func() {
		cancel()
		_ = command.Wait()
		_ = log.Close()
	})
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForForgejo(t, baseURL, logPath)

	client := New()
	client.BaseURL = baseURL
	client.TokenPath = filepath.Join(root, "soda-token")
	client.Binary = binary
	client.Config = configuration
	client.HTTP = &http.Client{Timeout: 10 * time.Second}
	client.run = directRunner
	person := domain.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Email: "alice@example.test"}
	user, err := client.EnsurePerson(context.Background(), person, PersonAdministrator)
	require.NoError(t, err)
	require.NotZero(t, user.ID)
	key, err := client.EnsureKey(context.Background(), person, domain.SSHDeviceKey{ID: "key-1", PublicKey: authorizedKey(t)})
	require.NoError(t, err)
	require.NotZero(t, key.ID)
	repository, err := client.EnsureRepository(context.Background(), domain.Project{ID: "project-1", Slug: "demo", Name: "Demo"}, []domain.Person{person}, authorizedKey(t))
	require.NoError(t, err)
	require.NotZero(t, repository.ID)
	require.NotZero(t, repository.DeployKeyID)
	require.Contains(t, repository.SSHURL, "soda/demo.git")
	exerciseRepository(t, root, repository.WebURL, person.Username, client.TokenPath)
}

func exerciseRepository(t *testing.T, root, remote, username, tokenPath string) {
	t.Helper()
	token, err := os.ReadFile(tokenPath)
	require.NoError(t, err)
	askpass := filepath.Join(root, "git-askpass")
	script := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\n  *Username*) printf '%%s\\n' %q ;;\n  *) printf '%%s\\n' %q ;;\nesac\n", username, strings.TrimSpace(string(token)))
	require.NoError(t, os.WriteFile(askpass, []byte(script), 0o700))
	environment := append(os.Environ(), "GIT_ASKPASS="+askpass, "GIT_TERMINAL_PROMPT=0")
	source := filepath.Join(root, "source")
	require.NoError(t, os.Mkdir(source, 0o755))
	runGit(t, environment, "-C", source, "init", "--initial-branch=main")
	runGit(t, environment, "-C", source, "config", "user.name", "Soda OS")
	runGit(t, environment, "-C", source, "config", "user.email", "soda@example.test")
	require.NoError(t, os.WriteFile(filepath.Join(source, "README.md"), []byte("# Built-in Git\n"), 0o644))
	runGit(t, environment, "-C", source, "add", "README.md")
	runGit(t, environment, "-C", source, "commit", "-m", "Initialize repository")
	runGit(t, environment, "-C", source, "remote", "add", "origin", remote+".git")
	runGit(t, environment, "-C", source, "push", "--set-upstream", "origin", "main")
	clone := filepath.Join(root, "clone")
	runGit(t, environment, "clone", remote+".git", clone)
	contents, err := os.ReadFile(filepath.Join(clone, "README.md"))
	require.NoError(t, err)
	require.Equal(t, "# Built-in Git\n", string(contents))
}

func runGit(t *testing.T, environment []string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Env = environment
	output, err := command.CombinedOutput()
	require.NoError(t, err, strings.TrimSpace(string(output)))
}

func directRunner(ctx context.Context, binary string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, binary, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func runForgejo(t *testing.T, binary, configuration string, args ...string) {
	t.Helper()
	command := append([]string{"--config", configuration}, args...)
	output, err := exec.Command(binary, command...).CombinedOutput()
	require.NoError(t, err, strings.TrimSpace(string(output)))
}

func waitForForgejo(t *testing.T, baseURL, logPath string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(baseURL + "/api/v1/version")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	contents, _ := os.ReadFile(logPath)
	t.Fatalf("Built-in Git did not become ready: %s", strings.TrimSpace(string(contents)))
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func authorizedKey(t *testing.T) string {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := ssh.NewPublicKey(public)
	require.NoError(t, err)
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
}

func testConfiguration(root string, port int) string {
	secret := strings.Repeat("a", 64)
	return fmt.Sprintf(`APP_NAME = Built-in Git
RUN_MODE = prod
RUN_USER = %s
WORK_PATH = %s

[database]
DB_TYPE = sqlite3
PATH = %s

[repository]
ROOT = %s
DEFAULT_PRIVATE = public

[server]
DOMAIN = localhost
HTTP_ADDR = 127.0.0.1
HTTP_PORT = %d
ROOT_URL = http://127.0.0.1:%d/
START_SSH_SERVER = false
SSH_USER = %s
SSH_ROOT_PATH = %s
SSH_CREATE_AUTHORIZED_KEYS_FILE = true
LFS_START_SERVER = false

[service]
DISABLE_REGISTRATION = true

[security]
INSTALL_LOCK = true
INTERNAL_TOKEN = %s
SECRET_KEY = %s

[oauth2]
JWT_SECRET = %s

[lfs]
JWT_SECRET = %s
`, os.Getenv("USER"), root, filepath.Join(root, "forgejo.db"), filepath.Join(root, "repositories"), port, port, os.Getenv("USER"), filepath.Join(root, ".ssh"), secret, secret, secret, secret)
}
