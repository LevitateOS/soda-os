#!/bin/sh
set -u

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
status=0

cd "$repository_root"

if ! ./scripts/check-cyclomatic-complexity.sh; then
	status=1
fi

echo "Checking cognitive and structural complexity..."
if ! go run github.com/mgechev/revive@v1.15.0 \
	-config revive.toml \
	-formatter friendly \
	-set_exit_status \
	-exclude './vendor/...' \
	-exclude './.artifacts/...' \
	-exclude '**/*.pb.go' \
	./...; then
	status=1
fi

echo "Checking struct field counts (maximum 10)..."
if ! go run ./tools/struct-field-limit; then
	status=1
fi

echo "Checking for complexity-lint suppressions..."
suppressions=$(find . \
	-type f \
	-name '*.go' \
	-not -name '*.pb.go' \
	-not -path './vendor/*' \
	-not -path './.artifacts/*' \
	-exec grep -nHE '//[[:space:]]*(revive:(disable|enable)|nolint|lint:ignore)' {} +)
if [ -n "$suppressions" ]; then
	printf '%s\n' "$suppressions"
	echo "Complexity-lint suppressions are forbidden." >&2
	status=1
fi

if [ "$status" -ne 0 ]; then
	echo "Complexity gates failed; commit and push are blocked." >&2
fi

exit "$status"
