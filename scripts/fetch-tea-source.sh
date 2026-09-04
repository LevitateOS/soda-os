#!/bin/sh
set -eu

if [ "$#" -ne 0 ]; then
  echo "usage: $0" >&2
  exit 2
fi

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
lock="$repo_root/distro/locks/tea-source.toml"

lock_value() {
  key=$1
  awk -v key="$key" '
    $0 ~ "^" key "[[:space:]]*=" {
      sub(/^[^=]*=[[:space:]]*"/, "")
      sub(/"[[:space:]]*$/, "")
      print
      exit
    }
  ' "$lock"
}

archive=$(lock_value source_archive)
url=$(lock_value source_url)
expected=$(lock_value source_sha256)
commit=$(lock_value commit)
license_sha256=$(lock_value license_sha256)

for value in "$archive" "$url" "$expected" "$commit" "$license_sha256"; do
  if [ -z "$value" ]; then
    echo "Tea source lock is incomplete" >&2
    exit 1
  fi
done

validate_digest() {
  if ! printf '%s\n' "$1" | grep -Eq '^[0-9a-f]{64}$'; then
    echo "Tea source lock contains an invalid SHA-256 digest" >&2
    exit 1
  fi
}

validate_digest "$expected"
if ! printf '%s\n' "$commit" | grep -Eq '^[0-9a-f]{40}$'; then
  echo "Tea source lock contains an invalid tagged commit" >&2
  exit 1
fi
validate_digest "$license_sha256"

if ! printf '%s\n' "$archive" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9._-]*\.gz$'; then
  echo "Tea source lock contains an invalid archive name" >&2
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

license="$repo_root/packaging/rpm/tea/sources/LICENSE"
if [ "$(checksum "$license")" != "$license_sha256" ]; then
  echo "Tea license checksum differs from the pinned upstream license" >&2
  exit 1
fi

output_directory="$repo_root/.artifacts/tools"
mkdir -p "$output_directory"
output="$output_directory/$archive"
if [ -f "$output" ] && [ "$(checksum "$output")" = "$expected" ] && tar -tzf "$output" >/dev/null; then
  exit 0
fi

temporary=$(mktemp "$output_directory/tea-source.XXXXXX")
trap 'rm -f "$temporary"' 0 1 2 15

curl --fail --location --output "$temporary" "$url"
actual=$(checksum "$temporary")
if [ "$actual" != "$expected" ]; then
  echo "Tea source checksum mismatch: got $actual, expected $expected" >&2
  exit 1
fi
tar -tzf "$temporary" >/dev/null
mv "$temporary" "$output"
trap - 0 1 2 15
