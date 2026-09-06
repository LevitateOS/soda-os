package acceptance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQualificationRequiresSameCoverageOnBothArchitectures(t *testing.T) {
	for architecture, platform := range map[string]string{"x86_64": "linux/amd64", "aarch64": "linux/arm64"} {
		t.Run(architecture, func(t *testing.T) {
			summary := qualificationFixture()
			summary.Architecture, summary.Platform = architecture, platform
			require.NoError(t, summary.Qualify())
			delete(summary.Scenarios, "public-ingress-rejection")
			require.NoError(t, summary.Validate(), "partial reports remain valid observations")
			require.ErrorContains(t, summary.Qualify(), "public-ingress-rejection")
		})
	}
}

func TestQualificationRejectsLegacyAndUnknownClaims(t *testing.T) {
	summary := qualificationFixture()
	summary.SchemaVersion = 1
	require.Error(t, summary.Validate())
	summary = qualificationFixture()
	summary.Scenarios["unreviewed-claim"] = "pass"
	require.Error(t, summary.Validate())
}
