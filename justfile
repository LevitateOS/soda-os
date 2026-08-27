set dotenv-load := false

default:
    @just --list

fmt:
    gofmt -w $(find . -name '*.go' -not -path './.artifacts/*')

check:
    test -z "$(gofmt -l $(find . -name '*.go' -not -path './.artifacts/*'))"
    ./scripts/protobuf-verify.sh
    go vet ./...
    go test ./...
    go run ./cmd/soda-image check

rpm:
    go run ./cmd/soda-image rpm

oci:
    go run ./cmd/soda-image oci
