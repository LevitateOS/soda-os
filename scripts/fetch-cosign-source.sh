#!/bin/sh
set -eu

if [ "$#" -ne 0 ]; then
  echo "usage: $0" >&2
  exit 2
fi

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
lock="$repo_root/distro/locks/cosign-source.toml"
lock_value() {
  awk -F '"' -v key="$1" '$1 ~ "^" key "[[:space:]]*=[[:space:]]*$" { print $2; exit }' "$lock"
}
archive=$(lock_value source_archive)
url=$(lock_value source_url)
expected=$(lock_value source_sha256)
if ! printf '%s\n' "$archive" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9._-]*\.tar\.gz$' ||
   ! printf '%s\n' "$expected" | grep -Eq '^[0-9a-f]{64}$' || [ -z "$url" ]; then
  echo "Cosign source lock is incomplete or invalid" >&2
  exit 1
fi
checksum() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}
output_directory="$repo_root/.artifacts/tools"
output="$output_directory/$archive"
if [ -f "$output" ] && [ "$(checksum "$output")" = "$expected" ] && tar -tzf "$output" >/dev/null; then
  exit 0
fi
mkdir -p "$output_directory"
temporary=$(mktemp "$output_directory/cosign-source.XXXXXX")
trap 'rm -f "$temporary"' 0 1 2 15
curl --fail --location --output "$temporary" "$url"
if [ "$(checksum "$temporary")" != "$expected" ]; then
  echo "Cosign source checksum mismatch" >&2
  exit 1
fi
tar -tzf "$temporary" >/dev/null
mv "$temporary" "$output"
trap - 0 1 2 15
