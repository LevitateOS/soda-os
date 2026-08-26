package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseID(t *testing.T) {
	id, err := ParseID("6fb70628-8ac7-45db-9d5b-bc08b885f66c")
	require.NoError(t, err)
	require.Equal(t, "6fb70628-8ac7-45db-9d5b-bc08b885f66c", id.String())

	_, err = ParseID("not-an-id")
	require.Error(t, err)
}

func TestProjectSourceVariants(t *testing.T) {
	require.NoError(t, ValidateProjectSource(EmptyProjectSource{}))
	require.NoError(t, ValidateProjectSource(GitProjectSource{RemoteURL: "git@example.com:team/project.git"}))
	require.Error(t, ValidateProjectSource(nil))
}
