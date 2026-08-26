set dotenv-load := false

default:
    @just --list

fmt:
    cargo fmt --all
    gofmt -w cockpit

check:
    cargo fmt --all --check
    cargo clippy --workspace --all-targets -- -D warnings
    cargo test --workspace
    go test ./cockpit/...
    cargo run -q -p soda-image -- check

rpm:
    cargo run -p soda-image -- rpm

iso:
    cargo run -p soda-image -- iso
