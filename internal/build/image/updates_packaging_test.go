package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdatesShipsThroughRuntimeWithNativeDependencies(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	spec, err := os.ReadFile(filepath.Join(root, "packaging/rpm/runtime/soda-runtime.spec"))
	require.NoError(t, err)
	for _, value := range []string{
		"/usr/bin/bootc", "/usr/bin/skopeo", "/usr/bin/cosign",
		"install -m 0755 %{_sourcedir}/soda-updates %{buildroot}%{_libexecdir}/soda/soda-updates",
		"%{_sourcedir}/soda-updates-cockpit/.", "%{_datadir}/cockpit/soda-updates/",
	} {
		require.Contains(t, string(spec), value)
	}
	staging, err := os.ReadFile(filepath.Join(root, "internal/build/image/rpm.go"))
	require.NoError(t, err)
	require.Contains(t, string(staging), `{"soda-updates", "./cmd/soda-updates"}`)
	require.Contains(t, string(staging), `{filepath.Join(build, "soda-updates"), filepath.Join(sources, "soda-updates")}`)
	sources := t.TempDir()
	require.NoError(t, (&Builder{Root: root}).stageCockpitSources(sources))
	assertCockpitStaged(t, root, sources, "soda-updates")
}
