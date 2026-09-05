package image

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type cosignSourceLock struct {
	Version       string `toml:"version"`
	Commit        string `toml:"commit"`
	SourceArchive string `toml:"source_archive"`
	SourceURL     string `toml:"source_url"`
	SourceSHA256  string `toml:"source_sha256"`
}

func readCosignSourceLock(path string) (cosignSourceLock, error) {
	var lock cosignSourceLock
	metadata, err := toml.DecodeFile(path, &lock)
	if err != nil {
		return lock, fmt.Errorf("read Cosign source lock: %w", err)
	}
	if len(metadata.Undecoded()) != 0 {
		return lock, errors.New("Cosign source lock contains unknown fields")
	}
	if !semanticVersionPattern.MatchString(lock.Version) || !validGitCommit(lock.Commit) ||
		!validInputFilename(lock.SourceArchive) || !strings.HasSuffix(lock.SourceArchive, ".tar.gz") ||
		lock.SourceURL == "" || !validSHA256(lock.SourceSHA256) {
		return lock, errors.New("Cosign source lock is incomplete or invalid")
	}
	return lock, nil
}

func (b *Builder) verifyCosignInput() error {
	lock, err := readCosignSourceLock(b.path("distro/locks/cosign-source.toml"))
	if err != nil {
		return err
	}
	if err := verifyFileSHA256(b.artifactPath("tools", lock.SourceArchive), lock.SourceSHA256); err != nil {
		return fmt.Errorf("verify Cosign source; run just cosign-source: %w", err)
	}
	return nil
}

func (b *Builder) buildCosign(ctx context.Context) error {
	if err := b.verifyCosignInput(); err != nil {
		return err
	}
	lock, err := readCosignSourceLock(b.path("distro/locks/cosign-source.toml"))
	if err != nil {
		return err
	}
	script := strings.Join([]string{
		"set -eu",
		"mkdir -p /src/.artifacts/build/cosign-source /src/.artifacts/build/cosign-go-cache /src/.artifacts/build/cosign-go-tmp",
		"tar -xzf /src/.artifacts/tools/" + lock.SourceArchive + " -C /src/.artifacts/build/cosign-source --strip-components=1",
		"cd /src/.artifacts/build/cosign-source",
		"make cosign GIT_VERSION=v" + lock.Version + " GIT_HASH=" + lock.Commit + " GIT_TREESTATE=clean",
		"install -m 0755 cosign /src/.artifacts/build/cosign",
		"install -m 0644 LICENSE /src/.artifacts/build/cosign-LICENSE",
		"/src/.artifacts/build/cosign version --json > /src/.artifacts/build/cosign-version.json",
		"grep -F '\"gitVersion\": \"v" + lock.Version + "\"' /src/.artifacts/build/cosign-version.json",
		"grep -F '\"gitCommit\": \"" + lock.Commit + "\"' /src/.artifacts/build/cosign-version.json",
	}, "\n")
	return b.docker(ctx, []string{
		"GOFLAGS=-buildvcs=false -mod=readonly",
		"GOTOOLCHAIN=local",
		"GOCACHE=/src/.artifacts/build/cosign-go-cache",
		"GOTMPDIR=/src/.artifacts/build/cosign-go-tmp",
		"SOURCE_DATE_EPOCH=" + fmt.Sprint(b.Spec.Build.SourceDateEpoch),
	}, "sh", "-c", script)
}
