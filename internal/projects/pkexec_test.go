package projects

import (
	"context"
	"testing"

	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/stretchr/testify/require"
)

func TestPKExecInvokerRequiresExplicitConstruction(t *testing.T) {
	_, err := (PKExecInvoker{}).CatalogAdd(context.Background(), catalog.Entry{
		ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git",
	})
	require.ErrorContains(t, err, "not constructed")
	_, err = NewPKExecInvoker("pkexec", "/usr/libexec/soda/soda-workspace-helper")
	require.ErrorContains(t, err, "absolute")
}
