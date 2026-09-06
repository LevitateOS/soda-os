package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForgejoOwnerPatchAndNativeFormWrappers(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	lock, err := readForgejoSourceLock(filepath.Join(root, "distro", "locks", "forgejo-source.toml"))
	require.NoError(t, err)
	require.NoError(t, verifyFileSHA256(filepath.Join(root, "packaging", "rpm", "forgejo", "sources", "patches", "0002-first-owner-registration.patch"), lock.OwnerPatchSHA256))
	templates := filepath.Join(root, "packaging", "rpm", "forgejo", "sources", "custom", "templates")
	for _, name := range []string{"signin", "signup"} {
		source, readErr := os.ReadFile(filepath.Join(templates, "user", "auth", name+".tmpl"))
		require.NoError(t, readErr)
		require.Contains(t, string(source), `{{template "custom/owner_setup" .}}`)
		require.Contains(t, string(source), `{{template "user/auth/`+name+`_inner" .}}`)
		require.NotContains(t, string(source), "<form", "native forms and validation must not be forked")
	}
	source, err := os.ReadFile(filepath.Join(templates, "custom", "owner_setup.tmpl"))
	require.NoError(t, err)
	for _, text := range []string{"Create your Forgejo administrator account", "SodaOwnerRegistrationAllowed", "independent Forgejo credentials", "missing-admin", "Registering another account cannot repair it"} {
		require.Contains(t, string(source), text)
	}
	welcome, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "sources", "console", "soda-console-welcome"))
	require.NoError(t, err)
	require.Contains(t, string(welcome), "First Forgejo setup:")
	require.Contains(t, string(welcome), "later humans sign in with Linux credentials")
}
