package projects

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

const credentialFileEnvironment = "SODA_GIT_CREDENTIAL_FILE"

type CloneCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sealedGitCredential struct {
	CloneCredentials
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
}

type Cloner interface {
	Clone(context.Context, string, string, CloneCredentials) error
}

type GitCloner struct {
	Binary string
	Stdout io.Writer
	Stderr io.Writer
}

func (cloner GitCloner) Clone(ctx context.Context, remote, destination string, credentials CloneCredentials) error {
	if err := validateCloneTarget(remote, destination); err != nil {
		return err
	}
	binary := cloner.Binary
	if binary == "" {
		binary = "/usr/bin/git"
	}
	command := cloner.cloneCommand(ctx, binary, remote, destination)
	secret, err := configureCloneCredentials(command, remote, credentials)
	if err != nil {
		return err
	}
	if secret != nil {
		defer secret.Close()
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("Git clone failed: %w", err)
	}
	return nil
}

func validateCloneTarget(remote, destination string) error {
	if err := ValidateCanonicalURL(remote); err != nil {
		return fmt.Errorf("clone URL: %w", err)
	}
	if destination == "" {
		return errors.New("clone destination is required")
	}
	_, err := os.Lstat(destination)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err == nil {
		return errors.New("clone destination already exists")
	}
	return fmt.Errorf("inspect clone destination: %w", err)
}

func (cloner GitCloner) cloneCommand(ctx context.Context, binary, remote, destination string) *exec.Cmd {
	command := exec.CommandContext(ctx, binary, "clone", "--", remote, destination)
	command.Stdout = cloner.Stdout
	if command.Stdout == nil {
		command.Stdout = os.Stdout
	}
	command.Stderr = cloner.Stderr
	if command.Stderr == nil {
		command.Stderr = os.Stderr
	}
	command.Env = environmentWithOverrides(os.Environ(), map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"LC_ALL":              "C",
	})
	return command
}

func configureCloneCredentials(command *exec.Cmd, remote string, credentials CloneCredentials) (*os.File, error) {
	if !isHTTPRemote(remote) {
		if credentials.Username != "" || credentials.Password != "" {
			return nil, errors.New("transient credentials are supported only for HTTP Git remotes")
		}
		return nil, nil
	}
	if credentials.Username == "" && credentials.Password == "" {
		return nil, nil
	}
	return configureAuthenticatedHTTPClone(command, remote, credentials)
}

func isHTTPRemote(remote string) bool {
	parsed, err := url.Parse(remote)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https"
}

func configureAuthenticatedHTTPClone(command *exec.Cmd, remote string, credentials CloneCredentials) (*os.File, error) {
	if credentials.Username == "" || credentials.Password == "" || strings.ContainsAny(credentials.Username+credentials.Password, "\x00\r\n") {
		return nil, errors.New("HTTP Git username and password are required")
	}
	secret, err := sealedCredentials(remote, credentials)
	if err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		secret.Close()
		return nil, fmt.Errorf("resolve Git credential executable: %w", err)
	}
	command.Env = authenticatedGitEnvironment(command.Env, executable)
	command.ExtraFiles = []*os.File{secret}
	return secret, nil
}

func authenticatedGitEnvironment(environment []string, executable string) []string {
	environment = sanitizeCredentialEnvironment(environment)
	return environmentWithOverrides(environment, map[string]string{
		credentialFileEnvironment: "/proc/self/fd/3",
		"GIT_CONFIG_COUNT":        "3",
		"GIT_CONFIG_KEY_0":        "credential.helper",
		"GIT_CONFIG_VALUE_0":      "",
		"GIT_CONFIG_KEY_1":        "credential.helper",
		"GIT_CONFIG_VALUE_1":      executable,
		"GIT_CONFIG_KEY_2":        "credential.interactive",
		"GIT_CONFIG_VALUE_2":      "false",
	})
}

func sealedCredentials(remote string, credentials CloneCredentials) (*os.File, error) {
	parsed, err := url.Parse(remote)
	if err != nil || parsed.Host == "" {
		return nil, errors.New("bind anonymous Git credential to remote")
	}
	sealed := sealedGitCredential{
		CloneCredentials: credentials,
		Protocol:         strings.ToLower(parsed.Scheme),
		Host:             strings.ToLower(parsed.Host),
	}
	descriptor, err := unix.MemfdCreate("soda-git-credentials", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return nil, fmt.Errorf("create anonymous Git credential: %w", err)
	}
	file := os.NewFile(uintptr(descriptor), "soda-git-credentials")
	if file == nil {
		unix.Close(descriptor)
		return nil, errors.New("open anonymous Git credential")
	}
	encoder := json.NewEncoder(file)
	if err = encoder.Encode(sealed); err != nil {
		file.Close()
		return nil, fmt.Errorf("write anonymous Git credential: %w", err)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return nil, fmt.Errorf("rewind anonymous Git credential: %w", err)
	}
	if _, err = unix.FcntlInt(file.Fd(), unix.F_ADD_SEALS, unix.F_SEAL_SEAL|unix.F_SEAL_SHRINK|unix.F_SEAL_GROW|unix.F_SEAL_WRITE); err != nil {
		file.Close()
		return nil, fmt.Errorf("seal anonymous Git credential: %w", err)
	}
	return file, nil
}

func environmentWithOverrides(environment []string, overrides map[string]string) []string {
	result := make([]string, 0, len(environment)+len(overrides))
	for _, value := range environment {
		name, _, _ := strings.Cut(value, "=")
		if _, replaced := overrides[name]; !replaced {
			result = append(result, value)
		}
	}
	for name, value := range overrides {
		result = append(result, name+"="+value)
	}
	return result
}

func sanitizeCredentialEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, value := range environment {
		name, _, _ := strings.Cut(value, "=")
		switch {
		case name == "GIT_CONFIG_PARAMETERS", name == "GIT_CURL_VERBOSE", name == "SSLKEYLOGFILE":
			continue
		case name == "GIT_ASKPASS", name == "GIT_ASKPASS_REQUIRE", name == "SSH_ASKPASS", name == "SSH_ASKPASS_REQUIRE":
			continue
		case strings.HasPrefix(name, "GIT_TRACE"):
			continue
		case name == "GIT_CONFIG_COUNT", strings.HasPrefix(name, "GIT_CONFIG_KEY_"), strings.HasPrefix(name, "GIT_CONFIG_VALUE_"):
			continue
		default:
			result = append(result, value)
		}
	}
	return result
}

func IsCredentialHelperInvocation() bool {
	return os.Getenv(credentialFileEnvironment) == "/proc/self/fd/3"
}

func RunCredentialHelper(operation string, input io.Reader, output io.Writer) error {
	if operation == "store" || operation == "erase" {
		return nil
	}
	if operation != "get" {
		return errors.New("Git credential operation is unsupported")
	}
	request, err := readCredentialRequest(input)
	if err != nil {
		return err
	}
	credential, err := readAnonymousCredential()
	if err != nil {
		return err
	}
	return writeMatchingCredential(request, credential, output)
}

func readCredentialRequest(input io.Reader) ([]byte, error) {
	request, err := io.ReadAll(io.LimitReader(input, 1<<20+1))
	if err != nil {
		return nil, errors.New("read Git credential request")
	}
	if len(request) > 1<<20 {
		return nil, errors.New("Git credential request exceeds 1 MiB")
	}
	return request, nil
}

func readAnonymousCredential() (sealedGitCredential, error) {
	path := os.Getenv(credentialFileEnvironment)
	if path != "/proc/self/fd/3" {
		return sealedGitCredential{}, errors.New("Git credential is unavailable")
	}
	file, err := os.Open(path)
	if err != nil {
		return sealedGitCredential{}, errors.New("open anonymous Git credential")
	}
	defer file.Close()
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return sealedGitCredential{}, errors.New("rewind anonymous Git credential")
	}
	var credential sealedGitCredential
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&credential); err != nil {
		return sealedGitCredential{}, errors.New("read anonymous Git credential")
	}
	return credential, nil
}

func writeMatchingCredential(request []byte, credential sealedGitCredential, output io.Writer) error {
	protocol, host, err := credentialRequestAuthority(request)
	if err != nil {
		return err
	}
	if protocol != credential.Protocol || host != credential.Host {
		return errors.New("Git credential request does not match the clone remote")
	}
	_, err = fmt.Fprintf(output, "username=%s\npassword=%s\n\n", credential.Username, credential.Password)
	return err
}

func credentialRequestAuthority(request []byte) (string, string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(request))
	fields := map[string]string{}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		if err := recordCredentialAuthority(fields, line); err != nil {
			return "", "", err
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", errors.New("read Git credential request")
	}
	if fields["protocol"] == "" || fields["host"] == "" {
		return "", "", errors.New("Git credential request is missing its authority")
	}
	return fields["protocol"], fields["host"], nil
}

func recordCredentialAuthority(fields map[string]string, line string) error {
	name, value, found := strings.Cut(line, "=")
	if !found || name == "" || strings.ContainsAny(name+value, "\x00\r\n") {
		return errors.New("Git credential request is malformed")
	}
	if name != "protocol" && name != "host" {
		return nil
	}
	if _, duplicate := fields[name]; duplicate {
		return errors.New("Git credential request repeats its authority")
	}
	fields[name] = strings.ToLower(value)
	return nil
}
