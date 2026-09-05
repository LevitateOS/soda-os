package acceptance

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Deliberately explicit: a newly required check must break qualification tests
// until its evidence and fixtures have been reviewed, not auto-pass in a loop.
func qualificationFixture() RunSummary {
	return RunSummary{
		SchemaVersion: 2, Architecture: "x86_64", Platform: "linux/amd64",
		SourceRevision: strings.Repeat("a", 40), SuiteRevision: strings.Repeat("a", 40),
		CandidateDigest: "sha256:" + strings.Repeat("c", 64),
		FallbackDigest:  "sha256:" + strings.Repeat("d", 64),
		CompletedAt:     SummaryTime(time.Unix(1_700_000_000, 0)),
		Scenarios: map[string]string{
			"iso-first-boot-defaults":            "pass",
			"qcow2-cloud-init-local":             "pass",
			"local-forwarded-access":             "pass",
			"tailnet-access":                     "pass",
			"installed-onboarding-observations":  "pass",
			"trusted-lan-access":                 "pass",
			"public-ingress-rejection":           "pass",
			"workspace-boundaries-and-git-keys":  "pass",
			"ssh-transports":                     "pass",
			"development-server-access":          "pass",
			"native-mise-ownership":              "pass",
			"workspace-removal":                  "pass",
			"cockpit-auth-and-independent-roles": "pass",
			"external-ssh-repository":            "pass",
			"project-removal":                    "pass",
			"human-removal-preserves-forgejo":    "pass",
			"update-and-fallback":                "pass",
			"packaged-boundaries":                "pass",
			"runner-completion":                  "pass",
			"evidence-and-cleanup":               "pass",
		},
	}
}

func TestPartialRunReportIsValidButCannotQualify(t *testing.T) {
	summary := qualificationFixture()
	require.NoError(t, summary.Qualify())
	delete(summary.Scenarios, "update-and-fallback")
	require.NoError(t, summary.Validate())
	require.ErrorContains(t, summary.Qualify(), "update-and-fallback")
	path := filepath.Join(t.TempDir(), "summary.json")
	require.NoError(t, WriteRunSummary(path, summary))
	actual, err := readRunSummary(path)
	require.NoError(t, err)
	require.Equal(t, summary, actual)
}

func TestEveryRequiredCheckMustBeEstablished(t *testing.T) {
	for _, name := range requiredChecks {
		t.Run(name, func(t *testing.T) {
			summary := qualificationFixture()
			delete(summary.Scenarios, name)
			require.ErrorContains(t, summary.Qualify(), name)
		})
	}
}

func TestRunReportRejectsInvalidClaims(t *testing.T) {
	for _, invalid := range []struct{ name, result string }{
		{"unknown", "pass"},
		{"lan-and-tailscale-access", "pass"},
		{"ssh-transports", "fail"},
		{"ssh-transports", "skip"},
	} {
		t.Run(invalid.name+"/"+invalid.result, func(t *testing.T) {
			summary := qualificationFixture()
			summary.Scenarios[invalid.name] = invalid.result
			require.Error(t, summary.Validate())
		})
	}
}

func TestForwardedAndTailnetAccessCannotQualifyMissingTopology(t *testing.T) {
	summary := qualificationFixture()
	delete(summary.Scenarios, "trusted-lan-access")
	delete(summary.Scenarios, "public-ingress-rejection")
	require.Equal(t, "pass", summary.Scenarios["local-forwarded-access"])
	require.Equal(t, "pass", summary.Scenarios["tailnet-access"])
	err := summary.Qualify()
	require.ErrorContains(t, err, "trusted-lan-access")
	require.ErrorContains(t, err, "public-ingress-rejection")
}

func TestRunReportRejectsObsoleteSchemaAndCandidateAsFallback(t *testing.T) {
	summary := qualificationFixture()
	summary.SchemaVersion = 1
	require.ErrorContains(t, summary.Validate(), "expected 2")
	summary.SchemaVersion = 2
	summary.FallbackDigest = summary.CandidateDigest
	require.ErrorContains(t, summary.Validate(), "must differ")
}

func TestCheckResultsRecordOnlySuccessfulObservations(t *testing.T) {
	results := checkResults{}
	require.NoError(t, results.record("ssh-transports", nil))
	require.Equal(t, checkResults{"ssh-transports": "pass"}, results)
	failure := errors.New("guest rejected command")
	require.ErrorIs(t, results.record("workspace-removal", failure), failure)
	require.NotContains(t, results, "workspace-removal")
	// Rechecking a claim unsuccessfully cannot leave an earlier pass behind.
	require.ErrorIs(t, results.record("ssh-transports", failure), failure)
	require.Empty(t, results)
}
