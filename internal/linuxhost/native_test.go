package linuxhost

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeRequiresExplicitConstruction(t *testing.T) {
	var native Native
	_, err := native.Run(context.Background(), Command{Name: "/usr/bin/true"})
	require.ErrorContains(t, err, "runner was not constructed")
	_, err = native.OpenAccountHome(Account{Username: "alice", Home: "/home/alice"})
	require.ErrorContains(t, err, "home root was not constructed")
}
