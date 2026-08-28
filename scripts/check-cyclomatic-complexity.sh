#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
maximum_complexity=10

cd "$repository_root"

echo "Checking cyclomatic complexity (maximum $maximum_complexity)..."
if ! go run github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0 \
	-over "$maximum_complexity" \
	-ignore '(^|/)(vendor|\.artifacts)/|\.pb\.go$' \
	.; then
	echo "Cyclomatic complexity exceeds $maximum_complexity; commit and push are blocked." >&2
	exit 1
fi
