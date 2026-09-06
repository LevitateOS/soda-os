package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/LevitateOS/soda-os/internal/projects"
	"github.com/stretchr/testify/require"
)

func TestIncompleteRemovalWritesReceiptBeforeFailure(t *testing.T) {
	for _, action := range []string{"remove", "remove-workspace", "delete-human"} {
		var output bytes.Buffer
		err := execute(t.Context(), []string{action}, nil, &output, func(context.Context, string, io.Reader) (any, error) {
			return projects.RemovalResponse{OK: false}, nil
		})
		require.ErrorIs(t, err, errIncompleteRemoval)
		var receipt projects.RemovalResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &receipt))
		require.False(t, receipt.OK)
	}
}

func TestCompleteRemovalSucceeds(t *testing.T) {
	err := execute(t.Context(), []string{"remove"}, nil, io.Discard, func(context.Context, string, io.Reader) (any, error) {
		return projects.RemovalResponse{OK: true}, nil
	})
	require.NoError(t, err)
}
