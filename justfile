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
    tests/acceptance/bootc.sh --help >/dev/null
    ./scripts/protobuf-verify.sh
    ./scripts/check-complexity.sh
    go vet ./...
    go test ./...
    go run ./cmd/soda-image --architecture aarch64 check
    go run ./cmd/soda-image --architecture x86_64 check

rpm architecture:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} rpm

oci architecture registry_ca public_key:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} oci --registry-ca {{quote(registry_ca)}} --public-key {{quote(public_key)}}

release-tools:
    ./scripts/fetch-release-tools.sh

publish architecture archive registry_ca public_key signing_key:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} publish --archive {{quote(archive)}} --registry-ca {{quote(registry_ca)}} --public-key {{quote(public_key)}} --signing-key {{quote(signing_key)}}

iso architecture image archive registry_ca public_key:
    go run ./cmd/soda-image --architecture {{quote(architecture)}} iso --image {{quote(image)}} --archive {{quote(archive)}} --registry-ca {{quote(registry_ca)}} --public-key {{quote(public_key)}}
