#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 aarch64|x86_64" >&2
  exit 2
fi

case "$1:$(uname -m)" in
  x86_64:x86_64|x86_64:amd64)
    platform=x86_64
    ;;
  aarch64:aarch64|aarch64:arm64)
    platform=aarch64
    ;;
  *)
    echo "Go builder inputs for $1 require matching-native hardware" >&2
    exit 1
    ;;
esac

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
lock="$repo_root/distro/platforms/$platform.toml"

builder_value() {
  key=$1
  awk -v key="$key" '
    /^\[builder\]$/ { in_builder = 1; next }
    in_builder && /^\[/ { exit }
    in_builder && $0 ~ "^" key "[[:space:]]*=" {
      sub(/^[^=]*=[[:space:]]*"/, "")
      sub(/"[[:space:]]*$/, "")
      print
      exit
    }
  ' "$lock"
}

version=$(builder_value go_version)
url=$(builder_value go_url)
archive=$(builder_value go_archive)
expected=$(builder_value go_archive_sha256)
for value in "$version" "$url" "$archive" "$expected"; do
  if [ -z "$value" ]; then
    echo "builder platform lock is incomplete for $platform" >&2
    exit 1
  fi
done
case "$archive" in
  .artifacts/tools/*.tar.gz) ;;
  *) echo "builder platform lock contains an invalid Go archive path" >&2; exit 1 ;;
esac
if ! printf '%s\n' "$expected" | grep -Eq '^[0-9a-f]{64}$'; then
  echo "builder platform lock contains an invalid Go checksum" >&2
  exit 1
fi

checksum() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo "a SHA-256 checksum utility is required" >&2
    exit 1
  fi
}

fetch_toolchain() {
  output="$repo_root/$archive"
  if [ -f "$output" ] && [ "$(checksum "$output")" = "$expected" ]; then
    return
  fi
  mkdir -p "$(dirname "$output")"
  curl --fail --location --output "$output.tmp" "$url"
  actual=$(checksum "$output.tmp")
  if [ "$actual" != "$expected" ]; then
    echo "Go toolchain checksum mismatch for $platform: got $actual, expected $expected" >&2
    exit 1
  fi
  mv "$output.tmp" "$output"
}

fetch_toolchain
