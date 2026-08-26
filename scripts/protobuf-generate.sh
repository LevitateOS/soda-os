#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tool_dir=$(mktemp -d "${TMPDIR:-/tmp}/soda-protobuf-tools.XXXXXX")
trap 'rm -rf "$tool_dir"' EXIT HUP INT TERM
export BUF_CACHE_DIR="${SODA_PROTOBUF_CACHE_DIR:-${TMPDIR:-/tmp}/soda-buf-cache}"

cd "$repository_root"
go build -o "$tool_dir/protoc-gen-go" google.golang.org/protobuf/cmd/protoc-gen-go
go build -o "$tool_dir/protoc-gen-go-grpc" google.golang.org/grpc/cmd/protoc-gen-go-grpc

if [ "$#" -eq 1 ]; then
	PATH="$tool_dir:$PATH" go tool buf generate --output "$1"
else
	PATH="$tool_dir:$PATH" go tool buf generate
fi
