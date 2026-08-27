set dotenv-load := false

default:
    @just --list

fmt:
    gofmt -w $(find . -name '*.go' -not -path './.artifacts/*')

check:
    test -z "$(gofmt -l $(find . -name '*.go' -not -path './.artifacts/*'))"
    sh -n scripts/bootc-acceptance.sh
    scripts/bootc-acceptance.sh --help >/dev/null
    ./scripts/protobuf-verify.sh
    go vet ./...
    go test ./...
    go run ./cmd/soda-image check

rpm:
    go run ./cmd/soda-image rpm

oci registry_ca public_key:
    go run ./cmd/soda-image oci --registry-ca {{quote(registry_ca)}} --public-key {{quote(public_key)}}

release-tools:
    ./scripts/fetch-release-tools.sh

publish archive registry_ca public_key signing_key:
    go run ./cmd/soda-image publish --archive {{quote(archive)}} --registry-ca {{quote(registry_ca)}} --public-key {{quote(public_key)}} --signing-key {{quote(signing_key)}}

iso image archive registry_ca public_key:
    go run ./cmd/soda-image iso --image {{quote(image)}} --archive {{quote(archive)}} --registry-ca {{quote(registry_ca)}} --public-key {{quote(public_key)}}
