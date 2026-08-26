#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
generated_root=$(mktemp -d "${TMPDIR:-/tmp}/soda-protobuf-generated.XXXXXX")
trap 'rm -rf "$generated_root"' EXIT HUP INT TERM
export BUF_CACHE_DIR="${SODA_PROTOBUF_CACHE_DIR:-${TMPDIR:-/tmp}/soda-buf-cache}"

cd "$repository_root"
go tool buf lint
./scripts/protobuf-generate.sh "$generated_root"
diff -u internal/gen/soda/v2/soda.pb.go "$generated_root/internal/gen/soda/v2/soda.pb.go"
diff -u internal/gen/soda/v2/soda_grpc.pb.go "$generated_root/internal/gen/soda/v2/soda_grpc.pb.go"
