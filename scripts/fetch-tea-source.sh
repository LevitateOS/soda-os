#!/bin/sh
set -eu

if [ "$#" -ne 0 ]; then
  echo "usage: $0" >&2
  exit 2
fi

case "$(uname -m)" in
  x86_64)
    architecture=x86_64
    ;;
  aarch64|arm64)
    architecture=aarch64
    ;;
  *)
    echo "unsupported native architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
lock="$repo_root/distro/locks/tea-source.toml"

root_value() {
  key=$1
  awk -v key="$key" '
    /^\[/ { exit }
    $0 ~ "^" key "[[:space:]]*=" {
      sub(/^[^=]*=[[:space:]]*"/, "")
      sub(/"[[:space:]]*$/, "")
      print
      exit
    }
  ' "$lock"
}

asset_value() {
  key=$1
  awk -v section="asset.$architecture" -v key="$key" '
    $0 == "[" section "]" { selected = 1; next }
    selected && /^\[/ { exit }
    selected && $0 ~ "^" key "[[:space:]]*=" {
      sub(/^[^=]*=[[:space:]]*"/, "")
      sub(/"[[:space:]]*$/, "")
      print
      exit
    }
  ' "$lock"
}

license_sha256=$(root_value license_sha256)
manifest_url=$(root_value checksum_manifest_url)
manifest_sha256=$(root_value checksum_manifest_sha256)
archive=$(asset_value archive)
url=$(asset_value url)
expected=$(asset_value sha256)

for value in "$license_sha256" "$manifest_url" "$manifest_sha256" "$archive" "$url" "$expected"; do
  if [ -z "$value" ]; then
    echo "Tea source lock is incomplete for $architecture" >&2
    exit 1
  fi
done

validate_digest() {
  if ! printf '%s\n' "$1" | grep -Eq '^[0-9a-f]{64}$'; then
    echo "Tea source lock contains an invalid SHA-256 digest" >&2
    exit 1
  fi
}

validate_digest "$license_sha256"
validate_digest "$manifest_sha256"
validate_digest "$expected"

case "$archive" in
  ''|*/*) echo "Tea source lock contains an invalid archive name" >&2; exit 1 ;;
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

license="$repo_root/packaging/rpm/tea/sources/LICENSE"
if [ "$(checksum "$license")" != "$license_sha256" ]; then
  echo "Tea license checksum differs from the pinned upstream license" >&2
  exit 1
fi

output_directory="$repo_root/.artifacts/tools"
mkdir -p "$output_directory"
output="$output_directory/$archive"
if [ -f "$output" ] && [ "$(checksum "$output")" = "$expected" ] && xz --test "$output"; then
  exit 0
fi

manifest_temporary=$(mktemp "$output_directory/tea-checksums.XXXXXX")
archive_temporary=$(mktemp "$output_directory/tea-archive.XXXXXX")
trap 'rm -f "$manifest_temporary" "$archive_temporary"' 0 1 2 15

curl --fail --location --output "$manifest_temporary" "$manifest_url"
if [ "$(checksum "$manifest_temporary")" != "$manifest_sha256" ]; then
  echo "Tea checksum manifest differs from the pinned upstream manifest" >&2
  exit 1
fi
if ! awk -v archive="$archive" -v expected="$expected" \
  '$1 == expected && $2 == archive { found = 1 } END { exit found ? 0 : 1 }' \
  "$manifest_temporary"; then
  echo "Tea checksum manifest does not bind $archive to its pinned digest" >&2
  exit 1
fi

curl --fail --location --output "$archive_temporary" "$url"
actual=$(checksum "$archive_temporary")
if [ "$actual" != "$expected" ]; then
  echo "Tea source checksum mismatch: got $actual, expected $expected" >&2
  exit 1
fi
xz --test "$archive_temporary"
mv "$archive_temporary" "$output"
trap - 0 1 2 15
rm -f "$manifest_temporary"
