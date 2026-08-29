#!/bin/sh
set -eu

version=15.0.7
expected=e11490f52542104651d81cfa7a23376a4c005397499e6dc1a7850e2fb8176ad6
output=".artifacts/tools/forgejo-src-${version}.tar.gz"

checksum() {
  sha256sum "$1" | awk '{print $1}'
}

if [ -f "$output" ] && [ "$(checksum "$output")" = "$expected" ]; then
  exit 0
fi

mkdir -p "$(dirname "$output")"
curl --fail --location --output "$output.tmp" "https://codeberg.org/forgejo/forgejo/releases/download/v${version}/forgejo-src-${version}.tar.gz"
actual=$(checksum "$output.tmp")
if [ "$actual" != "$expected" ]; then
  echo "Forgejo source checksum mismatch: got $actual, expected $expected" >&2
  exit 1
fi
mv "$output.tmp" "$output"
