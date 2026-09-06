set dotenv-load := false

default:
    @just --list

fmt:
    gofmt -w $(find . -name '*.go' -not -path './.artifacts/*')

complexity:
    ./scripts/check-complexity.sh

hooks-install:
    git config --local core.hooksPath .githooks

check:
    test -z "$(gofmt -l $(find . -name '*.go' -not -path './.artifacts/*'))"
    go run ./cmd/soda-acceptance --help >/dev/null
    ./scripts/check-complexity.sh
    ./scripts/check-release-identity.sh
    just cockpit-check
    go vet ./...
    go test -race ./internal/acceptance
    go test ./...
    go run ./cmd/soda-image --architecture aarch64 check
    go run ./cmd/soda-image --architecture x86_64 check

rpm architecture: forgejo-source (github-runner architecture) mise-rpm tea-source cosign-source
    go run ./cmd/soda-image --architecture {{quote(architecture)}} rpm

forgejo-source:
    ./scripts/fetch-forgejo-source.sh

mise-rpm:
    ./scripts/fetch-mise-rpm.sh

tea-source:
    ./scripts/fetch-tea-source.sh

cosign-source:
    ./scripts/fetch-cosign-source.sh

github-runner architecture:
    ./scripts/fetch-github-runner.sh {{quote(architecture)}}

oci architecture output_dir=".artifacts/images": forgejo-source (github-runner architecture) mise-rpm tea-source cosign-source
    go run ./cmd/soda-image --architecture {{quote(architecture)}} oci --output-dir {{quote(output_dir)}}

iso architecture archive:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} iso --archive {{quote(archive)}}

qcow2 architecture archive:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} qcow2 --archive {{quote(archive)}}

# Builds and publishes one native OCI; does not construct installers or update VMs.
dev-image architecture:
    ./scripts/prepare-native-image.sh {{quote(architecture)}}

# Run the complete shared frontend lifecycle before Go packaging tests.
cockpit-check:
    vp -C cockpit install --frozen-lockfile
    vp -C cockpit check
    vp -C cockpit build
    vp -C cockpit test
