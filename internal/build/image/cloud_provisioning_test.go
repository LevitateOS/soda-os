package image

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudProvisioningIsLimitedToTheTwoLocalCloudInitSources(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	config, err := os.ReadFile(filepath.Join(root, "packaging/rpm/runtime/sources/cloud/99-soda-datasources.cfg"))
	require.NoError(t, err)
	require.Equal(t, "datasource_list: [ NoCloud, ConfigDrive ]\n", string(config))

	finalizerPath := filepath.Join(root, "packaging/rpm/runtime/sources/soda-cloud-finalize")
	info, err := os.Stat(finalizerPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	check := exec.Command("python3", "-c", "import ast,pathlib,sys; ast.parse(pathlib.Path(sys.argv[1]).read_text())", finalizerPath)
	output, err := check.CombinedOutput()
	require.NoErrorf(t, err, "invalid cloud finalizer:\n%s", output)
	contents, err := os.ReadFile(finalizerPath)
	require.NoError(t, err)
	for _, expected := range []string{
		`INPUT_DIR = Path("/var/lib/soda-install/cloud")`,
		`TAILSCALE_KEY = TAILSCALE_DIR / "tailscale-auth-key"`,
		`SHADOW_PATH = Path("/etc/shadow")`,
		`Path("/var/lib/cloud/instances")`,
		`Path("/var/log/cloud-init.log")`,
		`def _remove_cloud_init_input_state():`,
		`"soda-cloud-finalize accepts no arguments and must run as root"`,
		`def _register_forgejo_public_key(username, password, ssh_key):`,
		`"/api/v1/user/keys"`,
		`["/usr/bin/systemctl", action, "soda-tailscale-enroll.service"]`,
	} {
		require.Contains(t, string(contents), expected)
	}
	for _, forbidden := range []string{"Restart=", "requests", "boto", "metadata.google", "aws", `"--password",`} {
		require.NotContains(t, string(contents), forbidden)
	}
	require.NotContains(t, string(contents), "spwd")
}

func TestRuntimePackageOwnsTheFixedCloudFinalizerOnly(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	spec, err := os.ReadFile(filepath.Join(root, "packaging/rpm/runtime/soda-runtime.spec"))
	require.NoError(t, err)
	for _, expected := range []string{
		`install -m 0755 %{_sourcedir}/soda-cloud-finalize`,
		`%{_libexecdir}/soda/soda-cloud-finalize`,
		`99-soda-datasources.cfg`,
	} {
		require.Contains(t, string(spec), expected)
	}
	require.NotContains(t, string(spec), "soda-cloud.service")

	containerfile, err := os.ReadFile(filepath.Join(root, "packaging/bootc/Containerfile"))
	require.NoError(t, err)
	require.Contains(t, string(containerfile), "cloud-init-local.service cloud-init-main.service cloud-config.service cloud-final.service")
	for _, architecture := range []string{"aarch64", "x86_64"} {
		builder, buildErr := NewBuilder(root, "distro/soda.toml", architecture, nil)
		require.NoError(t, buildErr)
		lock, lockErr := builder.packageLock()
		require.NoError(t, lockErr)
		require.Contains(t, lock.Package, lockedPackage{
			Name: "cloud-init", NEVRA: "cloud-init-0:26.1-1.fc44.noarch", Source: "fedora",
		})
	}
}
