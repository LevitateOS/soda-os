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
lock="$repo_root/distro/locks/bun-source.toml"

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
archive=$(asset_value archive)
member=$(asset_value member)
url=$(asset_value url)
expected=$(asset_value sha256)

for value in "$license_sha256" "$archive" "$member" "$url" "$expected"; do
  if [ -z "$value" ]; then
    echo "Bun source lock is incomplete for $architecture" >&2
    exit 1
  fi
done

validate_digest() {
  if ! printf '%s\n' "$1" | grep -Eq '^[0-9a-f]{64}$'; then
    echo "Bun source lock contains an invalid SHA-256 digest" >&2
    exit 1
  fi
}

validate_digest "$license_sha256"
validate_digest "$expected"

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

license="$repo_root/packaging/rpm/bun/sources/LICENSE.md"
if [ "$(checksum "$license")" != "$license_sha256" ]; then
  echo "Bun license checksum differs from the pinned upstream notice" >&2
  exit 1
fi

output="$repo_root/.artifacts/tools/$archive"
if [ -f "$output" ] && [ "$(checksum "$output")" = "$expected" ]; then
  unzip -Z1 "$output" | grep -Fx "$member" >/dev/null
  exit 0
fi

mkdir -p "$(dirname "$output")"
temporary="$output.tmp"
curl --fail --location --output "$temporary" "$url"
actual=$(checksum "$temporary")
if [ "$actual" != "$expected" ]; then
  echo "Bun source checksum mismatch: got $actual, expected $expected" >&2
  exit 1
fi
if ! unzip -Z1 "$temporary" | grep -Fx "$member" >/dev/null; then
  echo "Bun source archive does not contain $member" >&2
  exit 1
fi
mv "$temporary" "$output"
