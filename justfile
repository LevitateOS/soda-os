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
    sh -n tests/acceptance/bootc.sh
    sh -n tests/acceptance/unattended.sh
    tests/acceptance/bootc.sh --help >/dev/null
    tests/acceptance/unattended.sh --help >/dev/null
    ./scripts/protobuf-verify.sh
    ./scripts/check-complexity.sh
    go vet ./...
    go test ./...
    go run ./cmd/soda-image --architecture aarch64 check
    go run ./cmd/soda-image --architecture x86_64 check

rpm architecture: builder-tools forgejo-source
    go run ./cmd/soda-image --architecture {{quote(architecture)}} rpm

forgejo-source:
    ./scripts/fetch-forgejo-source.sh

oci architecture: builder-tools
    go run ./cmd/soda-image --architecture {{quote(architecture)}} oci

builder-tools:
    ./scripts/fetch-builder-tools.sh

iso architecture archive:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} iso --archive {{quote(archive)}}

record architecture archive iso:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} record --archive {{quote(archive)}} --iso {{quote(iso)}}
