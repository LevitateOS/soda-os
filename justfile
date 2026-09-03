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
    sh -n tests/acceptance/unattended.sh
    sh -n tests/acceptance/internal/bootc.sh
    tests/acceptance/unattended.sh --help >/dev/null
    ./scripts/check-complexity.sh
    ./scripts/check-release-identity.sh
    node --test cockpit/soda-projects/*.test.mjs
    go vet ./...
    go test ./...
    go run ./cmd/soda-image --architecture aarch64 check
    go run ./cmd/soda-image --architecture x86_64 check

rpm architecture: (builder-tools architecture) forgejo-source bun-source tea-source
    go run ./cmd/soda-image --architecture {{quote(architecture)}} rpm

forgejo-source:
    ./scripts/fetch-forgejo-source.sh

bun-source:
    ./scripts/fetch-bun-source.sh

tea-source:
    ./scripts/fetch-tea-source.sh

oci architecture: (builder-tools architecture) forgejo-source bun-source tea-source
    go run ./cmd/soda-image --architecture {{quote(architecture)}} oci

builder-tools architecture:
    ./scripts/fetch-builder-tools.sh {{quote(architecture)}}

iso architecture archive:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} iso --archive {{quote(archive)}}

record architecture archive iso:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} record --archive {{quote(archive)}} --iso {{quote(iso)}}
