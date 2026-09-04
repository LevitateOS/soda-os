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
    ./scripts/check-release-ci.sh
    node --test cockpit/soda-projects/*.test.mjs
    go vet ./...
    go test -race ./internal/acceptance
    go test ./...
    go run ./cmd/soda-image --architecture aarch64 check
    go run ./cmd/soda-image --architecture x86_64 check

rpm architecture: (builder-tools architecture) forgejo-source mise-rpm tea-source
    go run ./cmd/soda-image --architecture {{quote(architecture)}} rpm

forgejo-source:
    ./scripts/fetch-forgejo-source.sh

mise-rpm:
    ./scripts/fetch-mise-rpm.sh

tea-source:
    ./scripts/fetch-tea-source.sh

oci architecture: (builder-tools architecture) forgejo-source mise-rpm tea-source
    go run ./cmd/soda-image --architecture {{quote(architecture)}} oci

builder-tools architecture:
    ./scripts/fetch-builder-tools.sh {{quote(architecture)}}

iso architecture archive:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} iso --archive {{quote(archive)}}

qcow2 architecture archive:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} qcow2 --archive {{quote(archive)}}

record architecture archive iso qcow2 qcow2_zst:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} record --archive {{quote(archive)}} --iso {{quote(iso)}} --qcow2 {{quote(qcow2)}} --qcow2-zst {{quote(qcow2_zst)}}
