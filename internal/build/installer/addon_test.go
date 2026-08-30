package installer

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSodaIdentityAddonUsesSupportedDiscoveryAndPublicHandoff(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	addon := filepath.Join(root, "packaging", "installer", "addons", "org_fedoraproject_soda")
	required := []string{
		"__init__.py", "constants.py", "service/__main__.py", "service/identity.py",
		"service/interface.py", "service/installation.py", "service/kickstart.py",
		"gui/spokes/identity.py", "gui/spokes/identity.glade",
	}
	for _, name := range required {
		_, err := os.Stat(filepath.Join(addon, name))
		require.NoErrorf(t, err, "missing Anaconda add-on file %s", name)
	}

	containerfile, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "Containerfile"))
	require.NoError(t, err)
	require.Contains(t, string(containerfile), "COPY packaging/installer/addons/org_fedoraproject_soda /usr/share/anaconda/addons/org_fedoraproject_soda")
	require.Contains(t, string(containerfile), "org.fedoraproject.Anaconda.Addons.SodaIdentity.service")
	require.Contains(t, string(containerfile), "org.fedoraproject.Anaconda.Addons.SodaIdentity.conf")

	spoke, err := os.ReadFile(filepath.Join(addon, "gui", "spokes", "identity.py"))
	require.NoError(t, err)
	for _, expected := range []string{"UserSettingsCategory", "get_user_list", "users[0].gecos", "EMAIL.match", "def mandatory", "return True"} {
		require.Contains(t, string(spoke), expected)
	}

	installation, err := os.ReadFile(filepath.Join(addon, "service", "installation.py"))
	require.NoError(t, err)
	for _, expected := range []string{`"username": username`, `"name": name`, `"email": email`, `os.chmod(temporary, 0o600)`, `temporary.replace(path)`} {
		require.Contains(t, string(installation), expected)
	}
	require.NotContains(t, strings.ToLower(string(installation)), "password")

	glade, err := os.Open(filepath.Join(addon, "gui", "spokes", "identity.glade"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, glade.Close()) })
	var document struct{}
	require.NoError(t, xml.NewDecoder(glade).Decode(&document))

	var pythonFiles []string
	require.NoError(t, filepath.WalkDir(addon, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr == nil && !entry.IsDir() && filepath.Ext(path) == ".py" {
			pythonFiles = append(pythonFiles, path)
		}
		return walkErr
	}))
	for _, path := range pythonFiles {
		check := exec.Command("python3", "-c", "import ast, pathlib, sys; ast.parse(pathlib.Path(sys.argv[1]).read_text())", path)
		output, checkErr := check.CombinedOutput()
		require.NoErrorf(t, checkErr, "invalid Python in %s:\n%s", path, output)
	}
}

func TestInstallerAdminFirstBootHandoffIsConditionedAndPackaged(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	unit, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "sources", "systemd", "soda-installer-import.service"))
	require.NoError(t, err)
	for _, expected := range []string{
		"ConditionPathExists=/var/lib/soda/installer-admin.json",
		"Requires=sodad.service", "After=sodad.service",
		"ExecStart=/usr/bin/sodactl people import-installer --file /var/lib/soda/installer-admin.json",
	} {
		require.Contains(t, string(unit), expected)
	}
	preset, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "sources", "systemd", "90-soda.preset"))
	require.NoError(t, err)
	require.Contains(t, string(preset), "enable soda-installer-import.service")
}
