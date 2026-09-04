package acceptance

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSudoCommandPreservesEmptyPromptAcrossSSHCommandJoin(t *testing.T) {
	command := strings.Join(sudoScriptCommand(), " ")
	require.Equal(t, "sudo -k -S -p '' /bin/bash -eu -o pipefail -s", command)
}
