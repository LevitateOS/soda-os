package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectSourceVariants(t *testing.T) {
	require.NoError(t, ValidateProjectSource(EmptyProjectSource{}))
	require.NoError(t, ValidateProjectSource(GitProjectSource{RemoteURL: "git@example.com:team/project.git"}))
	require.Error(t, ValidateProjectSource(nil))
}
