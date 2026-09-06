package acceptance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAcceptanceHostPortsAreDistinct(t *testing.T) {
	require.NoError(t, validateHostPorts(HostPorts{SSH: 2222, Cockpit: 19090, Forgejo: 13000, Registry: 5001}))
	require.ErrorContains(t, validateHostPorts(HostPorts{SSH: 2222, Cockpit: 2222, Forgejo: 13000, Registry: 5001}), "distinct")
}
