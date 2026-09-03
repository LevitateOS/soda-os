#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repository_root"

go run github.com/mgechev/revive@v1.15.0 \
	-config revive.toml \
	-formatter friendly \
	-set_exit_status \
	-exclude './vendor/...' \
	-exclude './.artifacts/...' \
	-exclude '**/*.pb.go' \
	./...
