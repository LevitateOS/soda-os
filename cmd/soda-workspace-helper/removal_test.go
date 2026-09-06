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

func TestIncompleteRemovalIsTransportSuccess(t *testing.T) {
	for _, action := range []string{"project-remove", "workspace-remove", "human-delete"} {
		var output bytes.Buffer
		err := execute(t.Context(), []string{action}, nil, &output, func(context.Context, string, io.Reader) (any, error) {
			return projects.RemovalResponse{OK: false}, nil
		})
		require.NoError(t, err)
		var receipt projects.RemovalResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &receipt))
		require.False(t, receipt.OK)
	}
}
