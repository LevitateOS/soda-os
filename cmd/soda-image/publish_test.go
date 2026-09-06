package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/build/oci/ocitest"
	"github.com/LevitateOS/soda-os/internal/process"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/require"
)

type publicationBoundaryRunner struct {
	commands []process.Command
}

func (r *publicationBoundaryRunner) Run(context.Context, process.Command) error {
	return errors.New("unexpected mutation")
}

func (r *publicationBoundaryRunner) Output(_ context.Context, command process.Command) (string, error) {
	r.commands = append(r.commands, command)
	return "", errors.New("injected registry lookup failure")
}

func TestNativePublicationLoadsOnlySelectedSpecAndUsesInjectedRunner(t *testing.T) {
	architecture := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	root := t.TempDir()
	for _, name := range []string{"soda.toml", "platforms/" + architecture + ".toml"} {
		contents, err := os.ReadFile(filepath.Join("../../distro", name))
		require.NoError(t, err)
		path := filepath.Join(root, "distro", name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, contents, 0o644))
	}
	runner := &publicationBoundaryRunner{}
	builder, err := nativeBuilder(root, "distro/soda.toml", architecture, runner)
	require.NoError(t, err, "no sibling spec, installer lock, Git checkout or fetched build inputs exist here")
	img := ocitest.Image(t, &v1.ConfigFile{OS: "linux", Architecture: runtime.GOARCH, Config: v1.Config{Labels: map[string]string{
		"org.opencontainers.image.version":   "0.2.0",
		"org.opencontainers.image.revision":  strings.Repeat("a", 40),
		"org.opencontainers.image.base.name": "quay.io/fedora/fedora-bootc@sha256:" + strings.Repeat("b", 64),
	}}})
	_, err = builder.PublishImage(t.Context(), ocitest.Archive(t, img, runtime.GOARCH))
	require.ErrorContains(t, err, "injected registry lookup failure")
	require.Len(t, runner.commands, 1)
	require.Equal(t, "skopeo", runner.commands[0].Name)
	require.Equal(t, "list-tags", runner.commands[0].Args[0])
}
