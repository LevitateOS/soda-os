package acceptance

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunSummaryRequiresEveryPassingScenario(t *testing.T) {
	summary := RunSummary{
		SchemaVersion: 1, Architecture: "x86_64", Platform: "linux/amd64",
		SourceRevision: strings.Repeat("a", 40), SuiteRevision: strings.Repeat("a", 40),
		CandidateDigest: "sha256:" + strings.Repeat("b", 64),
		FallbackDigest:  "sha256:" + strings.Repeat("c", 64),
		Scenarios:       passedScenarios(), CompletedAt: SummaryTime(time.Unix(1_700_000_000, 0)),
	}
	require.NoError(t, summary.Validate())
	delete(summary.Scenarios, "update-and-fallback")
	require.ErrorContains(t, summary.Validate(), "incomplete scenario set")
}
