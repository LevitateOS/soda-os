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

iso:
    go run ./cmd/soda-image iso

iso-test:
    go run ./cmd/soda-image iso --automated

verify-iso:
    go run ./cmd/soda-image verify
