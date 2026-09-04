package acceptance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type HostPorts struct {
	SSH      int
	Cockpit  int
	Forgejo  int
	Registry int
}

type AdministratorInput struct {
	Username   string
	PrivateKey string
	PublicKey  string
	Password   string
}

type RunOptions struct {
	EvidenceDir    string
	Candidate      ArtifactSet
	Fallback       ArtifactSet
	TailscaleKey   string
	Administrator  AdministratorInput
	TempDir        string
	DiskSize       string
	Ports          HostPorts
	RepositoryRoot string
}

type RunResult struct {
	SummaryPath string
	EvidenceDir string
}

type runPaths struct {
	work           string
	adminKey       string
	adminPublicKey string
	password       string
	people         string
	installedDisk  string
	qcowDisk       string
	knownHosts     string
}

type runnerState struct {
	options   RunOptions
	artifacts ValidatedArtifacts
	evidence  Evidence
	paths     runPaths
	cleanup   *Cleanup
	registry  Registry
	tailnet   Tailnet
	secrets   []Secret
	output    io.Writer
	logout    func(context.Context) error
}

func Run(ctx context.Context, options RunOptions, output io.Writer) (RunResult, error) {
	state, err := newRunnerState(ctx, options, output)
	if err != nil {
		return RunResult{}, err
	}
	runErr := state.execute(ctx)
	secrets := state.secrets
	cleanupErr := state.cleanup.Run(context.Background())
	resultErr := errors.Join(runErr, cleanupErr)
	if resultErr != nil {
		_ = state.evidence.Write("failure.txt", []byte(resultErr.Error()+"\n"))
	}
	sanitizeErr := state.evidence.Sanitize(secrets)
	if resultErr = errors.Join(resultErr, sanitizeErr); resultErr != nil {
		return RunResult{EvidenceDir: state.evidence.Root}, resultErr
	}
	if err = state.writeSummary(ctx); err != nil {
		_ = state.evidence.Write("failure.txt", []byte(err.Error()+"\n"))
		return RunResult{EvidenceDir: state.evidence.Root}, err
	}
	summary := filepath.Join(state.evidence.Root, "summary.json")
	return RunResult{SummaryPath: summary, EvidenceDir: state.evidence.Root}, nil
}

func (state *runnerState) execute(ctx context.Context) error {
	if err := state.verifyFallbackPublication(ctx); err != nil {
		return fmt.Errorf("previous published fallback: %w", err)
	}
	if err := state.prepareRegistry(ctx); err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	scenario, vm, err := state.installAndOnboard(ctx)
	if err != nil {
		return fmt.Errorf("network ISO and first boot: %w", err)
	}
	if err = state.exerciseInstalledSystem(ctx, &scenario, &vm); err != nil {
		return err
	}
	if err = state.exerciseReusableQCOW2(ctx); err != nil {
		return fmt.Errorf("reusable QCOW2: %w", err)
	}
	return nil
}

func (state *runnerState) verifyFallbackPublication(ctx context.Context) error {
	command := fallbackVerificationCommand(state.artifacts.Fallback.SodaImageReference)
	output, err := CommandOutput(ctx, command)
	if err != nil {
		return err
	}
	return state.evidence.Write("fallback/published-signature.json", output)
}

func fallbackVerificationCommand(reference string) CommandSpec {
	return CommandSpec{Name: "cosign", Args: []string{
		"verify", "--certificate-identity", "https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production",
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com", reference,
	}}
}

func (state *runnerState) writeSummary(ctx context.Context) error {
	revision, err := state.repositoryRevision(ctx)
	if err != nil {
		return err
	}
	summary := RunSummary{
		SchemaVersion: 1, Architecture: nativeArchitecture(), Platform: state.artifacts.Candidate.Platform,
		SourceRevision: state.artifacts.Candidate.SourceRevision, SuiteRevision: revision,
		CandidateDigest: imageDigest(state.artifacts.Candidate), FallbackDigest: imageDigest(state.artifacts.Fallback),
		Scenarios: passedScenarios(), CompletedAt: SummaryTime(time.Now()),
	}
	return WriteRunSummary(filepath.Join(state.evidence.Root, "summary.json"), summary)
}

func (state *runnerState) repositoryRevision(ctx context.Context) (string, error) {
	output, err := CommandOutput(ctx, CommandSpec{Name: "git", Args: []string{"-C", state.options.RepositoryRoot, "rev-parse", "HEAD"}})
	if err != nil {
		return "", err
	}
	revision := strings.TrimSpace(string(output))
	if revision != state.artifacts.Candidate.SourceRevision {
		return "", fmt.Errorf("candidate source %s differs from acceptance suite revision %s", state.artifacts.Candidate.SourceRevision, revision)
	}
	return revision, nil
}

func nativeArchitecture() string {
	return map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
}

func imageDigest(record releaseRecord) string {
	return "sha256:" + strings.Split(record.SodaImageReference, "@sha256:")[1]
}
