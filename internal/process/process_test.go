package process

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOSRunnerWiresOnlyExplicitStdin(t *testing.T) {
	input := bytes.NewBufferString("administrator input")
	runner := OSRunner{Stdin: input}
	command := runner.command(context.Background(), Command{Name: "ignored"})
	require.Same(t, input, command.Stdin)
	require.Nil(t, (OSRunner{}).command(context.Background(), Command{Name: "ignored"}).Stdin)
}
