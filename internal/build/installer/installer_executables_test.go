package installer

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/installer_executables_test.py
var installerExecutableTests string

func TestInstallerExecutableBehavior(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)

	command := exec.Command(
		"python3",
		"-",
		filepath.Join(root, "packaging", "installer", "soda-installer-input"),
		filepath.Join(root, "packaging", "installer", "soda-installer-finalize"),
	)
	command.Stdin = strings.NewReader(installerExecutableTests)
	command.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "installer executable behavior tests failed:\n%s", output)
}
