package image

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/build/oci"
	"github.com/LevitateOS/soda-os/internal/build/oci/ocitest"
	"github.com/LevitateOS/soda-os/internal/process"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/require"
)

type publicationRunner struct {
	commands []process.Command
	digest   string
	tags     string
	failAt   int
	wrongAt  int
}

func (r *publicationRunner) Run(ctx context.Context, command process.Command) error {
	r.commands = append(r.commands, command)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(r.commands) == r.failAt {
		return errors.New("registry unavailable")
	}
	return nil
}

func (r *publicationRunner) Output(ctx context.Context, command process.Command) (string, error) {
	if err := r.Run(ctx, command); err != nil {
		return "", err
	}
	if command.Args[0] == "list-tags" {
		return r.tags, nil
	}
	if len(r.commands) == r.wrongAt {
		return "sha256:" + strings.Repeat("f", 64), nil
	}
	return r.digest + "\n", nil
}

func publicationFixture(t *testing.T, architecture string) (*Builder, *publicationRunner, string) {
	t.Helper()
	builder, _ := ociBuilder(t, architecture)
	builder.hostArchitecture = builder.Spec.Platform.Architecture.OCI
	img := ocitest.Image(t, &v1.ConfigFile{OS: "linux", Architecture: builder.hostArchitecture, Config: v1.Config{Labels: map[string]string{
		"org.opencontainers.image.version":   "0.2.0",
		"org.opencontainers.image.revision":  strings.Repeat("a", 40),
		"org.opencontainers.image.base.name": "quay.io/fedora/fedora-bootc@sha256:" + strings.Repeat("b", 64),
	}}})
	digest, err := img.Digest()
	require.NoError(t, err)
	runner := &publicationRunner{digest: digest.String(), tags: `{"Tags":[]}`}
	builder.runner = runner
	return builder, runner, ocitest.Archive(t, img, builder.hostArchitecture)
}

func TestPublicationUsesArchiveProvenanceAndOnlyNativeTag(t *testing.T) {
	for _, architecture := range []string{"aarch64", "x86_64"} {
		t.Run(architecture, func(t *testing.T) {
			builder, runner, archive := publicationFixture(t, architecture)
			image, err := builder.PublishImage(t.Context(), archive)
			require.NoError(t, err)
			require.Equal(t, "0.2.0", image.Version, "publisher spec version must not replace archive identity")
			require.Equal(t, strings.Repeat("a", 40), image.Revision)
			require.Len(t, runner.commands, 5)
			revision := oci.Repository + ":sha-" + image.Revision + "-" + architecture
			dev := oci.Repository + ":dev-" + architecture
			expected := [][]string{
				{"list-tags", "--tls-verify=true", "docker://" + oci.Repository},
				{"copy", "--preserve-digests", "--dest-tls-verify=true", "oci-archive:" + archive, "docker://" + revision},
				{"inspect", "--tls-verify=true", "--format", "{{.Digest}}", "docker://" + revision},
				{"copy", "--preserve-digests", "--src-tls-verify=true", "--dest-tls-verify=true", "docker://" + image.Reference(), "docker://" + dev},
				{"inspect", "--tls-verify=true", "--format", "{{.Digest}}", "--no-creds", "docker://" + dev},
			}
			for i, command := range runner.commands {
				require.Equal(t, "skopeo", command.Name, "no Git/installer/signature command")
				require.Equal(t, expected[i], command.Args)
			}
		})
	}
}

func TestPublicationAdvancesSameVersionFromDistinctArchiveBytes(t *testing.T) {
	builder, runner, archive := publicationFixture(t, "x86_64")
	first, err := builder.PublishImage(t.Context(), archive)
	require.NoError(t, err)
	img := ocitest.Image(t, &v1.ConfigFile{OS: "linux", Architecture: "amd64", Config: v1.Config{Labels: map[string]string{
		"org.opencontainers.image.version":   first.Version,
		"org.opencontainers.image.revision":  strings.Repeat("b", 40),
		"org.opencontainers.image.base.name": first.BaseReference,
	}}})
	digest, err := img.Digest()
	require.NoError(t, err)
	runner.commands, runner.digest = nil, digest.String()
	runner.tags = `{"Tags":["sha-` + first.Revision + `-x86_64"]}`
	second, err := builder.PublishImage(t.Context(), ocitest.Archive(t, img, "amd64"))
	require.NoError(t, err)
	require.Equal(t, first.Version, second.Version)
	require.NotEqual(t, first.Digest, second.Digest)
	require.Contains(t, runner.commands[1].Args, "docker://"+oci.Repository+":sha-"+second.Revision+"-x86_64")
	require.Contains(t, runner.commands[3].Args, "docker://"+second.Reference())
}

func TestPublicationExplicitRepublishAndConflicts(t *testing.T) {
	builder, runner, archive := publicationFixture(t, "x86_64")
	runner.tags = `{"Tags":["sha-` + strings.Repeat("a", 40) + `-x86_64"]}`
	_, err := builder.PublishImage(t.Context(), archive)
	require.NoError(t, err)
	require.Len(t, runner.commands, 4)
	require.Equal(t, "inspect", runner.commands[1].Args[0])
	require.Equal(t, "copy", runner.commands[2].Args[0])
	runner.commands, runner.wrongAt = nil, 2
	_, err = builder.PublishImage(t.Context(), archive)
	require.ErrorContains(t, err, "existing revision tag cannot be reused")
	require.Len(t, runner.commands, 2)
}

func TestPublicationFailuresStopAndDescribePartialEffects(t *testing.T) {
	for step, message := range []string{"before publication", "publication of", "verification failed", "advancement of", "anonymous verification failed"} {
		builder, runner, archive := publicationFixture(t, "x86_64")
		runner.failAt = step + 1
		image, err := builder.PublishImage(t.Context(), archive)
		require.ErrorContains(t, err, message)
		require.Empty(t, image.Digest)
		require.Len(t, runner.commands, step+1, "no automatic retries or later mutations")
	}
	for _, step := range []int{3, 5} {
		builder, runner, archive := publicationFixture(t, "x86_64")
		runner.wrongAt = step
		_, err := builder.PublishImage(t.Context(), archive)
		require.ErrorContains(t, err, "differs from archive digest")
		require.Len(t, runner.commands, step)
	}
}

func TestPublicationRejectsMalformedInputsBeforeMutation(t *testing.T) {
	for _, tags := range []string{`{}`, `{"Tags":null}`, `broken`} {
		builder, runner, archive := publicationFixture(t, "x86_64")
		runner.tags = tags
		_, err := builder.PublishImage(t.Context(), archive)
		require.Error(t, err)
		require.Len(t, runner.commands, 1)
	}
	builder, runner, archive := publicationFixture(t, "x86_64")
	builder.hostArchitecture = "arm64"
	_, err := builder.PublishImage(t.Context(), archive)
	require.ErrorContains(t, err, "native amd64 host")
	require.Empty(t, runner.commands)
	builder.hostArchitecture = "amd64"
	require.NoError(t, os.WriteFile(archive, []byte("not OCI"), 0o600))
	_, err = builder.PublishImage(t.Context(), archive)
	require.Error(t, err)
	require.Empty(t, runner.commands)
}
