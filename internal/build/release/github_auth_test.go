package release

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDraftRequiresAuthenticationAndRepositoryWritePermission(t *testing.T) {
	notes := filepath.Join(t.TempDir(), "notes.md")
	require.NoError(t, os.WriteFile(notes, []byte("release notes\n"), 0o644))

	t.Run("authentication", func(t *testing.T) {
		runner := &publicationRunner{revision: testRevision, failRunPrefix: "gh auth status"}
		_, err := testPublication(t, runner, "arm64").Draft(context.Background(), DraftOptions{NotesPath: notes})
		require.ErrorContains(t, err, "verify GitHub CLI authentication")
		require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "gh api repos/")
	})

	t.Run("permission", func(t *testing.T) {
		runner := &publicationRunner{
			revision: testRevision,
			states:   []string{repositoryResponseJSON(testRevision, repositoryResponseFixture{permission: "READ"})},
		}
		_, err := testPublication(t, runner, "arm64").Draft(context.Background(), DraftOptions{NotesPath: notes})
		require.ErrorContains(t, err, "requires write permission")
		require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "gh api repos/")
	})
}
