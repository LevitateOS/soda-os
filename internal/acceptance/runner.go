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
	work          string
	installedDisk string
	qcowDisk      string
	knownHosts    string
}

type runnerState struct {
	options   RunOptions
	artifacts ValidatedArtifacts
	evidence  Evidence
	paths     runPaths
	cleanup   *Cleanup
	registry  Registry
	checks    checkResults
	secrets   []Secret
	output    io.Writer
}

func Run(ctx context.Context, options RunOptions, output io.Writer) (RunResult, error) {
	state, err := newRunnerState(ctx, options, output)
	if err != nil {
		return RunResult{}, err
	}
	inputs, runErr := state.prepareInputs(ctx)
	if runErr == nil {
		runErr = state.execute(ctx, inputs)
	}
	return state.finish(ctx, runErr)
}

// finish retains partial reports even when execution or finalization fails.
func (state *runnerState) finish(ctx context.Context, runErr error) (RunResult, error) {
	runErr = state.checks.record("runner-completion", runErr)
	cleanupErr := state.cleanup.Run(context.Background())
	cleanupLog := "result=pass\n"
	if cleanupErr != nil {
		cleanupLog = "result=fail\n" + cleanupErr.Error() + "\n"
	}
	cleanupErr = errors.Join(cleanupErr, state.evidence.Write("cleanup.txt", []byte(cleanupLog)))
	resultErr := errors.Join(runErr, cleanupErr)
	if resultErr != nil {
		resultErr = errors.Join(resultErr, state.evidence.Write("failure.txt", []byte(resultErr.Error()+"\n")))
	}
	sanitizeErr := state.evidence.Sanitize(state.secrets)
	finalizeErr := state.checks.record("evidence-and-cleanup", errors.Join(cleanupErr, sanitizeErr))
	// A cancelled guest run must still be able to record its completed checks.
	reportCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	reportErr := state.writeSummary(reportCtx)
	result := RunResult{EvidenceDir: state.evidence.Root}
	if reportErr == nil {
		result.SummaryPath = filepath.Join(state.evidence.Root, "summary.json")
	}
	return result, redactError(errors.Join(resultErr, finalizeErr, reportErr), state.secrets)
}

func (state *runnerState) writeSummary(ctx context.Context) error {
	revision, err := state.repositoryRevision(ctx)
	if err != nil {
		return err
	}
	summary := RunSummary{
		SchemaVersion: 2, Architecture: nativeArchitecture(), Platform: state.artifacts.Candidate.Platform,
		SourceRevision: state.artifacts.Candidate.Revision, SuiteRevision: revision,
		CandidateDigest: state.artifacts.Candidate.Digest, FallbackDigest: state.artifacts.Fallback.Digest,
		Scenarios: state.checks, CompletedAt: SummaryTime(time.Now()),
	}
	return WriteRunSummary(filepath.Join(state.evidence.Root, "summary.json"), summary)
}

func (state *runnerState) repositoryRevision(ctx context.Context) (string, error) {
	output, err := CommandOutput(ctx, CommandSpec{Name: "git", Args: []string{"-C", state.options.RepositoryRoot, "rev-parse", "HEAD"}})
	if err != nil {
		return "", err
	}
	revision := strings.TrimSpace(string(output))
	if revision != state.artifacts.Candidate.Revision {
		return "", fmt.Errorf("candidate source %s differs from acceptance suite revision %s", state.artifacts.Candidate.Revision, revision)
	}
	return revision, nil
}

func nativeArchitecture() string {
	return map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
}
