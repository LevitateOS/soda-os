package acceptance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

func newRunnerState(ctx context.Context, options RunOptions, output io.Writer) (*runnerState, error) {
	options = defaultRunOptions(options)
	if err := validateRunOptions(options); err != nil {
		return nil, err
	}
	artifacts, err := ValidateArtifacts(options.Candidate, options.Fallback)
	if err != nil {
		return nil, err
	}
	if err = requireCleanSource(ctx, options.RepositoryRoot, artifacts.Candidate.SourceRevision); err != nil {
		return nil, err
	}
	evidence, err := CreateEvidence(options.EvidenceDir)
	if err != nil {
		return nil, err
	}
	state := &runnerState{options: options, artifacts: artifacts, evidence: evidence, cleanup: &Cleanup{}, output: output}
	if err = state.prepareInputs(ctx); err != nil {
		_ = evidence.Write("failure.txt", []byte(err.Error()+"\n"))
		cleanupErr := state.cleanup.Run(context.Background())
		sanitizeErr := evidence.Sanitize(state.secrets)
		return nil, errors.Join(err, cleanupErr, sanitizeErr)
	}
	return state, nil
}

func defaultRunOptions(options RunOptions) RunOptions {
	if options.Administrator.Username == "" {
		options.Administrator.Username = "soda-test"
	}
	if options.TempDir == "" {
		options.TempDir = os.Getenv("RUNNER_TEMP")
	}
	if options.DiskSize == "" {
		options.DiskSize = "40G"
	}
	if options.Ports.SSH == 0 {
		options.Ports.SSH = 2222
	}
	if options.Ports.Cockpit == 0 {
		options.Ports.Cockpit = 19090
	}
	if options.Ports.Forgejo == 0 {
		options.Ports.Forgejo = 13000
	}
	if options.Ports.Registry == 0 {
		options.Ports.Registry = 5001
	}
	if options.RepositoryRoot == "" {
		options.RepositoryRoot = "."
	}
	return options
}

func validateRunOptions(options RunOptions) error {
	if options.EvidenceDir == "" || options.TailscaleKey == "" {
		return errors.New("evidence, reusable ephemeral Tailscale key, and disposable administrator credential files are required")
	}
	if err := validateAdministratorInput(options.Administrator); err != nil {
		return err
	}
	if err := validateHostPorts(options.Ports); err != nil {
		return err
	}
	if err := validateCredentialFiles(options); err != nil {
		return err
	}
	return RequireCommands("cosign", "curl", "docker", "git", "qemu-img", "ssh", "ssh-keygen", "ssh-keyscan")
}

func validateCredentialFiles(options RunOptions) error {
	if err := requireProtectedSecret(options.TailscaleKey); err != nil {
		return fmt.Errorf("Tailscale auth key: %w", err)
	}
	if _, err := readSecretLine(options.TailscaleKey); err != nil {
		return fmt.Errorf("Tailscale auth key: %w", err)
	}
	if err := requireProtectedSecret(options.Administrator.PrivateKey); err != nil {
		return fmt.Errorf("administrator private key: %w", err)
	}
	if err := requireProtectedSecret(options.Administrator.Password); err != nil {
		return fmt.Errorf("administrator password: %w", err)
	}
	if _, err := readSecretLine(options.Administrator.Password); err != nil {
		return fmt.Errorf("administrator password: %w", err)
	}
	if err := requireRegularFile(options.Administrator.PublicKey); err != nil {
		return fmt.Errorf("administrator public key: %w", err)
	}
	return nil
}

func validateAdministratorInput(input AdministratorInput) error {
	if !usernamePattern.MatchString(input.Username) {
		return errors.New("administrator username must match [a-z][a-z0-9-]{0,23}")
	}
	if input.PrivateKey == "" || input.PublicKey == "" || input.Password == "" {
		return errors.New("disposable administrator credential files are required")
	}
	return nil
}

func validateHostPorts(ports HostPorts) error {
	seen := map[int]bool{}
	for _, port := range []int{ports.SSH, ports.Cockpit, ports.Forgejo, ports.Registry} {
		if port < 1 || port > 65535 {
			return errors.New("host ports must be inside the TCP range")
		}
		if seen[port] {
			return errors.New("host ports must be distinct")
		}
		seen[port] = true
	}
	return nil
}

func requireProtectedSecret(path string) error {
	if err := requireRegularFile(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("secret file %s must be mode 0600 or stricter", path)
	}
	return nil
}

func readSecretLine(path string) ([]byte, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	value := bytes.TrimSuffix(contents, []byte{'\n'})
	if len(value) == 0 || bytes.ContainsAny(value, "\x00\r\n") {
		return nil, errors.New("secret file must contain one non-empty line without NUL, CR, or LF")
	}
	return value, nil
}

func requireCleanSource(ctx context.Context, root, revision string) error {
	actual, err := CommandOutput(ctx, CommandSpec{Name: "git", Args: []string{"-C", root, "rev-parse", "HEAD"}})
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(actual)) != revision {
		return errors.New("candidate release record does not name the runner checkout")
	}
	status, err := CommandOutput(ctx, CommandSpec{Name: "git", Args: []string{"-C", root, "status", "--porcelain=v1", "--untracked-files=normal"}})
	if err != nil {
		return err
	}
	if len(status) != 0 {
		return errors.New("acceptance runner checkout has tracked changes")
	}
	return nil
}

func (state *runnerState) prepareInputs(ctx context.Context) error {
	work, err := os.MkdirTemp(state.options.TempDir, "soda-acceptance-")
	if err != nil {
		return err
	}
	state.paths = runPaths{
		work: work, adminKey: state.options.Administrator.PrivateKey, adminPublicKey: state.options.Administrator.PublicKey,
		password: state.options.Administrator.Password, people: filepath.Join(work, "people"),
		installedDisk: filepath.Join(work, "installed.qcow2"), qcowDisk: filepath.Join(work, "reusable.qcow2"),
		knownHosts: filepath.Join(work, "known-hosts"),
	}
	if err = state.cleanup.Add(CleanupAction{Name: "generated work directory " + work, Run: func(context.Context) error { return os.RemoveAll(work) }}); err != nil {
		return err
	}
	if err = os.Mkdir(state.paths.people, 0o700); err != nil {
		return err
	}
	password, err := readSecretLine(state.paths.password)
	if err != nil {
		return err
	}
	if err = validateAdministratorKeyPair(ctx, state.paths.adminKey, state.paths.adminPublicKey); err != nil {
		return err
	}
	return state.loadSecrets(password)
}

func validateAdministratorKeyPair(ctx context.Context, privatePath, publicPath string) error {
	derived, err := CommandOutput(ctx, CommandSpec{Name: "ssh-keygen", Args: []string{"-y", "-f", privatePath}})
	if err != nil {
		return fmt.Errorf("derive administrator public key: %w", err)
	}
	provided, err := os.ReadFile(publicPath)
	if err != nil {
		return err
	}
	derivedKey, err := canonicalPublicKey(derived)
	if err != nil {
		return err
	}
	providedKey, err := canonicalPublicKey(provided)
	if err != nil {
		return err
	}
	if derivedKey != providedKey {
		return errors.New("administrator SSH private and public keys do not match")
	}
	return nil
}

func (state *runnerState) loadSecrets(password []byte) error {
	privateKey, err := os.ReadFile(state.paths.adminKey)
	if err != nil {
		return err
	}
	tailscaleKey, err := os.ReadFile(state.options.TailscaleKey)
	if err != nil {
		return err
	}
	state.secrets = []Secret{
		{Label: "tailscale-auth-key", Value: tailscaleKey},
		{Label: "administrator-password", Value: password},
		{Label: "administrator-private-key", Value: privateKey},
	}
	return nil
}

func (state *runnerState) prepareRegistry(ctx context.Context) error {
	docker, err := SelectDocker(ctx)
	if err != nil {
		return err
	}
	registryImage, err := ReadPinnedImage(filepath.Join(state.options.RepositoryRoot, "tests/acceptance/registry-image.txt"))
	if err != nil {
		return err
	}
	skopeoImage, err := ReadPinnedImage(filepath.Join(state.options.RepositoryRoot, "tests/acceptance/skopeo-image.txt"))
	if err != nil {
		return err
	}
	state.registry = Registry{Docker: docker, Name: RegistryName(time.Now()), Port: state.options.Ports.Registry, Data: filepath.Join(state.paths.work, "registry"), Evidence: state.evidence}
	if err = state.registry.Start(ctx, registryImage); err != nil {
		return err
	}
	if err = state.cleanup.Add(CleanupAction{Name: "container " + state.registry.Name, Run: state.registry.Stop}); err != nil {
		return err
	}
	return state.publishImages(ctx, skopeoImage)
}

func (state *runnerState) publishImages(ctx context.Context, skopeoImage string) error {
	for _, item := range []struct{ archive, tag, digest string }{
		{state.artifacts.FallbackOCI, "fallback", imageDigest(state.artifacts.Fallback)},
		{state.artifacts.CandidateOCI, "candidate", imageDigest(state.artifacts.Candidate)},
	} {
		actual, err := state.registry.Publish(ctx, item.archive, item.tag, skopeoImage)
		if err != nil {
			return err
		}
		if actual != item.digest {
			return fmt.Errorf("registry changed %s manifest digest from %s to %s", item.tag, item.digest, actual)
		}
	}
	return nil
}
