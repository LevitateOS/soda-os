package image

import (
	"context"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

// The exporter runner creates only a text stand-in, never an OCI artifact or a
// native process. These tests exercise command/path contracts for both platforms.
type ociRunner struct {
	t        *testing.T
	commands []process.Command
	failAt   int
	failure  error
	missing  bool
}

func (r *ociRunner) Run(ctx context.Context, command process.Command) error {
	r.commands = append(r.commands, command)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(r.commands) == r.failAt {
		return r.failure
	}
	if command.Args[0] == "buildx" && !r.missing {
		fields, err := csv.NewReader(strings.NewReader(command.Args[slices.Index(command.Args, "--output")+1])).Read()
		require.NoError(r.t, err)
		require.Equal(r.t, []string{"type=oci", "oci-mediatypes=true", "rewrite-timestamp=true"}, []string{fields[0], fields[2], fields[3]})
		path := strings.TrimPrefix(fields[1], "dest=")
		_, err = os.Lstat(path)
		require.True(r.t, os.IsNotExist(err), "selected previous output must be removed before export")
		return os.WriteFile(path, []byte("simulated export"), 0o644)
	}
	return nil
}

func (r *ociRunner) Output(context.Context, process.Command) (string, error) {
	r.t.Fatal("OCI export must not query another source revision or prepare inputs again")
	return "", nil
}

func ociBuilder(t *testing.T, architecture string) (*Builder, *ociRunner) {
	t.Helper()
	root, err := filepath.Abs("../../..")
	require.NoError(t, err)
	spec, err := config.LoadDistro(filepath.Join(root, "distro/soda.toml"), architecture)
	require.NoError(t, err)
	runner := &ociRunner{t: t, failure: errors.New("native command failed")}
	return &Builder{Root: t.TempDir(), Spec: spec, runner: runner}, runner
}

func TestOCIArchiveDestinationsAndIdentity(t *testing.T) {
	for _, architecture := range []string{"aarch64", "x86_64"} {
		for _, destination := range []string{"", ".artifacts/images/" + architecture + "/" + strings.Repeat("a", 40), "spaces, and \"quotes\"", t.TempDir()} {
			t.Run(architecture+"/"+destination, func(t *testing.T) {
				builder, runner := ociBuilder(t, architecture)
				directory := destination
				if directory == "" {
					directory = ".artifacts/images"
				}
				expected := filepath.Join(builder.path(directory), "soda-os-"+builder.Spec.Identity.Version+"-"+architecture+".oci.tar")
				require.NoError(t, os.MkdirAll(filepath.Dir(expected), 0o755))
				require.NoError(t, os.WriteFile(expected, []byte("previous export"), 0o644))
				inputs := imageBuildInputs{revision: strings.Repeat("a", 40), baseTag: "verified-base"}
				path, err := builder.buildOCIArchive(t.Context(), inputs, destination)
				require.NoError(t, err)
				require.Equal(t, expected, path)
				require.FileExists(t, path)
				require.Len(t, runner.commands, 3)
				build := runner.commands[0]
				require.Equal(t, builder.Root, build.Dir)
				require.Equal(t, "docker", build.Name)
				require.Equal(t, builder.Spec.Base.Platform, build.Args[slices.Index(build.Args, "--platform")+1])
				require.Subset(t, build.Args, []string{
					"SODA_VERSION=" + builder.Spec.Identity.Version,
					"SODA_SOURCE_REVISION=" + inputs.revision,
					"fedora-base=docker-image://verified-base",
					"rpm-inputs=" + builder.artifactPath("rpms"),
					"lock-inputs=" + builder.artifactPath("bootc"),
					"FEDORA_BASE_REFERENCE=" + builder.Spec.Base.Reference,
					"BOOTC_NEVRA=" + builder.Spec.Platform.Base.BootcNEVRA,
					"--provenance=false",
				})
				require.Equal(t, []string{"load", "--input", path}, runner.commands[1].Args)
				require.Equal(t, []string{"run", "--rm", "--platform", builder.Spec.Base.Platform, "--entrypoint", "bootc", builder.Spec.Image.Registry + ":" + builder.Spec.Identity.Version, "container", "lint"}, runner.commands[2].Args)
			})
		}
	}
}

func TestOCIArchivesAtSameVersionKeepOtherRevisions(t *testing.T) {
	builder, _ := ociBuilder(t, "x86_64")
	var paths []string
	for _, revision := range []string{strings.Repeat("a", 40), strings.Repeat("b", 40)} {
		directory := filepath.Join(".artifacts/images/x86_64", revision)
		path, err := builder.buildOCIArchive(t.Context(), imageBuildInputs{revision: revision, baseTag: "base"}, directory)
		require.NoError(t, err)
		paths = append(paths, path)
		require.NoError(t, os.WriteFile(path, []byte(revision), 0o644))
	}
	require.NotEqual(t, paths[0], paths[1])
	contents, err := os.ReadFile(paths[0])
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("a", 40), string(contents))
	require.Equal(t, filepath.Base(paths[0]), filepath.Base(paths[1]))
}

func TestOCIArchiveCommandFailuresReturnNoPath(t *testing.T) {
	for _, failAt := range []int{1, 2, 3} {
		t.Run([]string{"export", "load", "lint"}[failAt-1], func(t *testing.T) {
			builder, runner := ociBuilder(t, "x86_64")
			runner.failAt = failAt
			path, err := builder.buildOCIArchive(t.Context(), imageBuildInputs{}, "")
			require.ErrorIs(t, err, runner.failure)
			require.Empty(t, path)
			require.Len(t, runner.commands, failAt)
		})
	}
}

func TestOCIArchiveMissingExportAndCancellation(t *testing.T) {
	builder, runner := ociBuilder(t, "x86_64")
	runner.missing = true
	path, err := builder.buildOCIArchive(t.Context(), imageBuildInputs{}, "")
	require.ErrorContains(t, err, "OCI export did not create an archive")
	require.Empty(t, path)
	require.Len(t, runner.commands, 1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	path, err = builder.buildOCIArchive(ctx, imageBuildInputs{}, "")
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, path)
	require.Len(t, runner.commands, 2)
}

func TestBuildImageInputFailureLeavesDestinationUntouched(t *testing.T) {
	builder, runner := ociBuilder(t, "x86_64")
	builder.hostArchitecture = "amd64"
	directory := filepath.Join(builder.Root, "chosen-output")
	path, err := builder.BuildImage(t.Context(), directory)
	require.ErrorContains(t, err, "parse package lock")
	require.Empty(t, path)
	require.NoDirExists(t, directory)
	require.Empty(t, runner.commands)
}

func TestOCIArchiveFilesystemFailuresBeforeExport(t *testing.T) {
	for _, obstructArchive := range []bool{false, true} {
		builder, runner := ociBuilder(t, "x86_64")
		destination := filepath.Join(builder.Root, "output")
		obstruction := destination
		if obstructArchive {
			obstruction = filepath.Join(destination, "soda-os-"+builder.Spec.Identity.Version+"-x86_64.oci.tar", "keep")
			require.NoError(t, os.MkdirAll(filepath.Dir(obstruction), 0o755))
		}
		require.NoError(t, os.WriteFile(obstruction, []byte("keep"), 0o644))
		path, err := builder.buildOCIArchive(t.Context(), imageBuildInputs{}, destination)
		require.Error(t, err)
		require.Empty(t, path)
		require.Empty(t, runner.commands)
		require.FileExists(t, obstruction)
	}
}
