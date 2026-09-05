package scripts

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestReleaseRequiresQualifiedSourceBeforeNativePreparation(t *testing.T) {
	contents, err := os.ReadFile("../.github/workflows/release.yml")
	require.NoError(t, err)
	var workflow struct {
		Jobs map[string]struct {
			Needs string                 `yaml:"needs"`
			Steps []struct{ Run string } `yaml:"steps"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(contents, &workflow))
	require.Equal(t, "validate", workflow.Jobs["prepare"].Needs)
	var verification string
	for _, step := range workflow.Jobs["validate"].Steps {
		if strings.Contains(step.Run, "soda-acceptance verify") {
			verification = step.Run
		}
	}
	require.Contains(t, verification, `--commit "$GITHUB_SHA"`)
	require.Contains(t, verification, `--branch main --event workflow_dispatch`)
	require.Contains(t, verification, `--expected-revision "$GITHUB_SHA"`)
	require.Contains(t, verification, `--name "soda-native-acceptance-$GITHUB_SHA"`)
	require.Contains(t, verification, "set -eu")
	require.NotContains(t, string(contents), "soda-acceptance run")
	require.NotContains(t, string(contents), "soda-acceptance record")
}
