package main

import (
	"context"
	"sort"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

func TestCommandExposesOnlyFixedPublicationOperations(t *testing.T) {
	command := newCommand(&commandRunner{})
	names := make([]string, 0, len(command.Commands()))
	for _, child := range command.Commands() {
		names = append(names, child.Name())
	}
	sort.Strings(names)
	require.Equal(t, []string{"draft", "image-promote", "image-stage", "publish", "upload"}, names)

	for _, forbidden := range []string{"github-token-env", "repository", "asset", "clobber", "output-dir"} {
		flag := command.PersistentFlags().Lookup(forbidden)
		require.Nil(t, flag)
	}
}

type commandRunner struct{}

func (*commandRunner) Run(_ context.Context, _ process.Command) error { return nil }

func (*commandRunner) Output(_ context.Context, _ process.Command) (string, error) { return "", nil }
