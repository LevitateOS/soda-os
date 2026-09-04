#!/bin/sh
set -eu

if [ "$#" -ne 0 ]; then
  echo "usage: $0" >&2
  exit 2
fi

case "$(uname -m)" in
  x86_64) architecture=x86_64 ;;
  aarch64|arm64) architecture=aarch64 ;;
  *) echo "unsupported native architecture: $(uname -m)" >&2; exit 1 ;;
esac

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
lock="$repo_root/distro/locks/mise-source.toml"

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

file=$(asset_value file)
url=$(asset_value url)
expected=$(asset_value sha256)
for value in "$file" "$url" "$expected"; do
  if [ -z "$value" ]; then
    echo "mise source lock is incomplete for $architecture" >&2
    exit 1
  fi
done
if ! printf '%s\n' "$expected" | grep -Eq '^[0-9a-f]{64}$'; then
  echo "mise source lock contains an invalid SHA-256 digest" >&2
  exit 1
fi

checksum() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

output="$repo_root/.artifacts/tools/$file"
if [ -f "$output" ] && [ "$(checksum "$output")" = "$expected" ]; then
  exit 0
fi
mkdir -p "$(dirname "$output")"
temporary="$output.tmp"
curl --fail --location --output "$temporary" "$url"
actual=$(checksum "$temporary")
if [ "$actual" != "$expected" ]; then
  echo "mise RPM checksum mismatch: got $actual, expected $expected" >&2
  exit 1
fi
mv "$temporary" "$output"
