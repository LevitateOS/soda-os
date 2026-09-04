package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/LevitateOS/soda-os/internal/projects"
	"golang.org/x/sys/unix"
)

const (
	forgejoConfig  = "/etc/forgejo/app.ini"
	forgejoBaseURL = "http://127.0.0.1:30000"
)

type forgejoUser struct {
	Username string
	Active   bool
	Admin    bool
}

type NativeForgejo struct {
	Runner projects.CommandRunner
	Client projects.ForgejoClient
}

func (forgejo NativeForgejo) runner() projects.CommandRunner {
	if forgejo.Runner == nil {
		return projects.ExecCommandRunner{}
	}
	return forgejo.Runner
}

func (forgejo NativeForgejo) Ready(ctx context.Context, username string) bool {
	users, err := forgejo.users(ctx)
	if err != nil {
		return false
	}
	for _, user := range users {
		if user.Username == username {
			return user.Active && user.Admin
		}
	}
	return false
}

func (forgejo NativeForgejo) PrepareAdministrator(ctx context.Context, request AdministratorRequest) error {
	users, err := forgejo.users(ctx)
	if err != nil {
		return err
	}
	bootstrap, err := forgejoAdministratorPreparation(users, request.Username)
	if err != nil {
		return err
	}
	if bootstrap {
		if err = forgejo.bootstrap(ctx, request); err != nil {
			return err
		}
	}
	if err = forgejo.waitHealthy(ctx); err != nil {
		return err
	}
	if err = forgejo.Client.RegisterPublicKey(ctx, projects.ForgejoKeyRequest{
		BaseURL: forgejoBaseURL, Username: request.Username, Password: request.Password, PublicKey: strings.TrimSpace(request.AuthorizedKey),
	}); err != nil {
		return fmt.Errorf("Forgejo administrator %s was retained without the requested public key: %w", request.Username, err)
	}
	if !forgejo.Ready(ctx, request.Username) {
		return errors.New("Forgejo did not report the requested administrator as ready")
	}
	return nil
}

func forgejoAdministratorPreparation(users []forgejoUser, username string) (bool, error) {
	if len(users) == 0 {
		return true, nil
	}
	if len(users) == 1 && users[0].Username == username && users[0].Active && users[0].Admin {
		return false, nil
	}
	return false, errors.New("Forgejo already contains an unexpected user; correct it with native Forgejo administration and reopen Soda Setup")
}

func (forgejo NativeForgejo) users(ctx context.Context) ([]forgejoUser, error) {
	result, err := forgejo.runner().Run(ctx, projects.Command{
		Name: "/usr/sbin/runuser", Args: []string{"--user", "git", "--", "/usr/bin/forgejo", "admin", "user", "list", "--config", forgejoConfig},
	})
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, errors.New("Forgejo user state is unavailable")
	}
	return parseForgejoUsers(result.Stdout)
}

func parseForgejoUsers(output string) ([]forgejoUser, error) {
	users := make([]forgejoUser, 0)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if _, err := strconv.ParseInt(fields[0], 10, 64); err != nil {
			continue
		}
		if len(fields) < 5 {
			return nil, errors.New("Forgejo returned an invalid user list")
		}
		users = append(users, forgejoUser{Username: fields[1], Active: fields[3] == "true", Admin: fields[4] == "true"})
	}
	return users, nil
}

func (forgejo NativeForgejo) bootstrap(ctx context.Context, request AdministratorRequest) (returnErr error) {
	configuration, err := os.ReadFile(forgejoConfig)
	if err != nil {
		return errors.New("Forgejo configuration is unavailable")
	}
	temporary, err := forgejoBootstrapConfiguration(configuration)
	if err != nil {
		return err
	}
	configFile, err := sealedForgejoConfiguration(temporary)
	if err != nil {
		return err
	}
	defer configFile.Close()
	if err = forgejo.systemctl(ctx, "stop", "forgejo.service"); err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, forgejo.systemctl(context.Background(), "start", "forgejo.service"))
	}()

	command := exec.CommandContext(ctx, "/usr/sbin/runuser", "--user", "git", "--", "/usr/bin/forgejo", "web", "--config", "/proc/self/fd/3")
	command.ExtraFiles = []*os.File{configFile}
	command.Stdin = nil
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err = command.Start(); err != nil {
		return errors.New("start temporary Forgejo administrator setup")
	}
	defer func() { returnErr = errors.Join(returnErr, stopForgejoProcess(command)) }()
	if err = forgejo.waitHealthy(ctx); err != nil {
		return err
	}
	if err = submitForgejoSignup(ctx, request); err != nil {
		return err
	}
	return nil
}

func forgejoBootstrapConfiguration(configuration []byte) ([]byte, error) {
	registration := []byte("DISABLE_REGISTRATION = true")
	listener := []byte("HTTP_ADDR = 0.0.0.0")
	if bytes.Count(configuration, registration) != 1 || bytes.Count(configuration, listener) != 1 {
		return nil, errors.New("Forgejo bootstrap configuration is unavailable for initial administrator setup")
	}
	configuration = bytes.Replace(configuration, registration, []byte("DISABLE_REGISTRATION = false"), 1)
	return bytes.Replace(configuration, listener, []byte("HTTP_ADDR = 127.0.0.1"), 1), nil
}

func sealedForgejoConfiguration(configuration []byte) (*os.File, error) {
	descriptor, err := unix.MemfdCreate("soda-setup-forgejo", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), "soda-setup-forgejo")
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, errors.New("open anonymous Forgejo configuration descriptor")
	}
	if _, err = file.Write(configuration); err != nil {
		file.Close()
		return nil, err
	}
	if _, err = file.Seek(0, 0); err != nil {
		file.Close()
		return nil, err
	}
	seals := unix.F_SEAL_SEAL | unix.F_SEAL_SHRINK | unix.F_SEAL_GROW | unix.F_SEAL_WRITE
	if _, err = unix.FcntlInt(file.Fd(), unix.F_ADD_SEALS, seals); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func submitForgejoSignup(ctx context.Context, request AdministratorRequest) error {
	form := url.Values{
		"user_name": {request.Username},
		"email":     {request.Username + "@localhost"},
		"password":  {request.Password},
		"retype":    {request.Password},
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, forgejoBaseURL+"/user/sign_up", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Set("Sec-Fetch-Site", "same-origin")
	client, err := directLocalHTTPClient(10 * time.Second)
	if err != nil {
		return err
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return errors.New("Forgejo rejected administrator creation")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/" {
		return errors.New("Forgejo rejected administrator creation")
	}
	return nil
}

func (forgejo NativeForgejo) waitHealthy(ctx context.Context) error {
	deadline := time.Now().Add(30 * time.Second)
	client, err := directLocalHTTPClient(time.Second)
	if err != nil {
		return err
	}
	for time.Now().Before(deadline) {
		if forgejoHealthy(ctx, client) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return errors.New("Forgejo did not become ready")
}

func forgejoHealthy(ctx context.Context, client *http.Client) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, forgejoBaseURL+"/api/healthz", nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	return response.StatusCode == http.StatusOK
}

func directLocalHTTPClient(timeout time.Duration) (*http.Client, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("local HTTP transport is unavailable")
	}
	direct := transport.Clone()
	direct.Proxy = nil
	return &http.Client{
		Transport: direct,
		Timeout:   timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

func stopForgejoProcess(command *exec.Cmd) error {
	if command.Process == nil || command.ProcessState != nil {
		return nil
	}
	_ = command.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(10 * time.Second):
		_ = command.Process.Kill()
		<-done
		return nil
	}
}

func (forgejo NativeForgejo) systemctl(ctx context.Context, action, unit string) error {
	result, err := forgejo.runner().Run(ctx, projects.Command{Name: "/usr/bin/systemctl", Args: []string{action, unit}})
	if err != nil || result.ExitCode != 0 {
		return fmt.Errorf("%s %s", action, unit)
	}
	return nil
}
