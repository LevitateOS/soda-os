package acceptance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProductChecksStopBeforeClaimingIncompleteComposite(t *testing.T) {
	commands := filepath.Join(t.TempDir(), "commands")
	t.Setenv("COMMAND_LOG", commands)
	installAcceptanceCommand(t, "ssh", `cat >/dev/null
printf 'ssh\n' >>"$COMMAND_LOG"
test "$(wc -l <"$COMMAND_LOG")" -lt 2
`)
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	state := runnerState{evidence: evidence, checks: checkResults{"update-and-fallback": "pass"}}
	project := projectFixture{Admin: workspaceFixture{Person: personFixture{Remote: Remote{Evidence: evidence}}}}
	err = state.exerciseProductScenarios(context.Background(), project, fixtureKeys{}, "")
	require.ErrorContains(t, err, "workspace-boundaries-and-git-keys")
	require.Equal(t, checkResults{"update-and-fallback": "pass"}, state.checks)
	contents, err := os.ReadFile(commands)
	require.NoError(t, err)
	require.Equal(t, "ssh\nssh\n", string(contents))
}

func TestFinishRetainsPartialReportAfterCancelledExecution(t *testing.T) {
	state := reportingState(t)
	state.checks["ssh-transports"] = "pass"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := state.finish(ctx, context.Canceled)
	require.ErrorIs(t, err, context.Canceled)
	summary, err := readRunSummary(result.SummaryPath)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"ssh-transports": "pass", "evidence-and-cleanup": "pass"}, summary.Scenarios)
	require.ErrorContains(t, summary.Qualify(), "runner-completion")
}

func TestFinishCannotQualifyCleanupFailure(t *testing.T) {
	state := reportingState(t)
	failure := errors.New("fixture cleanup failed")
	require.NoError(t, state.cleanup.Add(CleanupAction{Name: "fixture", Run: func(context.Context) error { return failure }}))
	result, err := state.finish(context.Background(), nil)
	require.ErrorIs(t, err, failure)
	summary, err := readRunSummary(result.SummaryPath)
	require.NoError(t, err)
	require.Equal(t, "pass", summary.Scenarios["runner-completion"])
	require.NotContains(t, summary.Scenarios, "evidence-and-cleanup")
	require.ErrorContains(t, summary.Qualify(), "evidence-and-cleanup")
}

func TestFinishCannotQualifyRedactedEvidence(t *testing.T) {
	state := reportingState(t)
	state.secrets = []Secret{{Label: "test-password", Value: []byte("fixture-secret")}}
	require.NoError(t, state.evidence.Write("guest.log", []byte("fixture-secret")))
	result, err := state.finish(context.Background(), nil)
	require.ErrorContains(t, err, "credential material reached evidence")
	summary, err := readRunSummary(result.SummaryPath)
	require.NoError(t, err)
	require.NotContains(t, summary.Scenarios, "evidence-and-cleanup")
	contents, err := os.ReadFile(filepath.Join(state.evidence.Root, "guest.log"))
	require.NoError(t, err)
	require.Equal(t, "[REDACTED]", string(contents))
}

func TestFinishedRunnerStillCannotSupplyMissingObservations(t *testing.T) {
	state := reportingState(t)
	result, err := state.finish(context.Background(), nil)
	require.NoError(t, err)
	summary, err := readRunSummary(result.SummaryPath)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"runner-completion": "pass", "evidence-and-cleanup": "pass"}, summary.Scenarios)
	err = summary.Qualify()
	require.ErrorContains(t, err, "installed-onboarding-observations")
	require.ErrorContains(t, err, "trusted-lan-access")
	require.ErrorContains(t, err, "public-ingress-rejection")
}

func TestReportWriteFailureReturnsNoReportPath(t *testing.T) {
	state := reportingState(t)
	require.NoError(t, os.Mkdir(filepath.Join(state.evidence.Root, "summary.json"), 0o700))
	result, err := state.finish(context.Background(), nil)
	require.Error(t, err)
	require.Empty(t, result.SummaryPath)
	require.Equal(t, state.evidence.Root, result.EvidenceDir)
}

func reportingState(t *testing.T) *runnerState {
	t.Helper()
	revision := strings.Repeat("a", 40)
	installAcceptanceCommand(t, "git", "printf '%s\\n' "+revision+"\n")
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	return &runnerState{
		evidence: evidence, cleanup: &Cleanup{}, checks: checkResults{},
		artifacts: ValidatedArtifacts{
			Candidate: releaseRecord{SourceRevision: revision,
				Platform:           map[string]string{"x86_64": "linux/amd64", "aarch64": "linux/arm64"}[nativeArchitecture()],
				SodaImageReference: "image@sha256:" + strings.Repeat("c", 64)},
			Fallback: releaseRecord{SodaImageReference: "image@sha256:" + strings.Repeat("d", 64)},
		},
	}
}

func installAcceptanceCommand(t *testing.T, name, script string) {
	t.Helper()
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, name), []byte("#!/bin/sh\nset -eu\n"+script), 0o700))
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}
