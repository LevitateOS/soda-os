#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <aarch64|x86_64>" >&2
  exit 2
fi

architecture=$1
case "$architecture" in
  aarch64|x86_64) ;;
  *) echo "unsupported architecture: $architecture" >&2; exit 2 ;;
esac

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
lock="$repo_root/distro/locks/github-runner-source.toml"

asset_value() {
  key=$1
  awk -v architecture="$architecture" -v key="$key" '
    /^\[\[asset\]\]$/ { selected = 0 }
    /^architecture[[:space:]]*=/ {
      value = $0
      sub(/^[^=]*=[[:space:]]*"/, "", value)
      sub(/"[[:space:]]*$/, "", value)
      selected = value == architecture
    }
    selected && $0 ~ "^" key "[[:space:]]*=" {
      sub(/^[^=]*=[[:space:]]*"/, "")
      sub(/"[[:space:]]*$/, "")
      print
      exit
    }
  ' "$lock"
}

archive=$(asset_value archive)
url=$(asset_value url)
expected=$(asset_value sha256)
if [ -z "$archive" ] || [ -z "$url" ] || ! printf '%s\n' "$expected" | grep -Eq '^[0-9a-f]{64}$'; then
  echo "GitHub runner source lock is incomplete for $architecture" >&2
  exit 1
fi
case "$archive" in
  */*|''|*[!A-Za-z0-9._-]*) echo "GitHub runner source lock contains an invalid archive name" >&2; exit 1 ;;
esac

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

output_directory="$repo_root/.artifacts/tools"
mkdir -p "$output_directory"
output="$output_directory/$archive"
if [ -f "$output" ] && [ "$(checksum "$output")" = "$expected" ] && tar -tzf "$output" >/dev/null; then
  exit 0
fi

temporary=$(mktemp "$output_directory/github-runner.XXXXXX")
trap 'rm -f "$temporary"' 0 1 2 15
curl --fail --location --output "$temporary" "$url"
actual=$(checksum "$temporary")
if [ "$actual" != "$expected" ]; then
  echo "GitHub runner checksum mismatch: got $actual, expected $expected" >&2
  exit 1
fi
tar -tzf "$temporary" >/dev/null
mv "$temporary" "$output"
trap - 0 1 2 15
