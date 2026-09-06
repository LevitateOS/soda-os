#!/bin/sh
set -eu

if [ "$#" -ne 0 ]; then
  echo "usage: $0" >&2
  exit 2
fi

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
lock="$repo_root/distro/locks/forgejo-source.toml"

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
url=$(lock_value url)
expected=$(lock_value sha256)
patch_sha256=$(lock_value patch_sha256)

for value in "$archive" "$url" "$expected" "$patch_sha256"; do
  if [ -z "$value" ]; then
    echo "Forgejo source lock is incomplete" >&2
    exit 1
  fi
done

case "$archive" in
  *.tar.gz) ;;
  *) echo "Forgejo source lock contains an invalid archive name" >&2; exit 1 ;;
esac
case "$archive" in
  */*) echo "Forgejo source lock contains an invalid archive name" >&2; exit 1 ;;
esac

validate_digest() {
  if ! printf '%s\n' "$1" | grep -Eq '^[0-9a-f]{64}$'; then
    echo "Forgejo source lock contains an invalid SHA-256 digest" >&2
    exit 1
  fi
}

validate_digest "$expected"
validate_digest "$patch_sha256"

output="$repo_root/.artifacts/tools/$archive"

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

patch="$repo_root/packaging/rpm/forgejo/sources/patches/0001-pam-do-not-retain-password.patch"
if [ "$(checksum "$patch")" != "$patch_sha256" ]; then
  echo "Forgejo PAM patch checksum differs from the source lock" >&2
  exit 1
fi

if [ -f "$output" ] && [ "$(checksum "$output")" = "$expected" ] && tar -tzf "$output" >/dev/null; then
  exit 0
fi

mkdir -p "$(dirname "$output")"
temporary=$(mktemp "$(dirname "$output")/forgejo-source.XXXXXX")
trap 'rm -f "$temporary"' 0 1 2 15
curl --fail --location --output "$temporary" "$url"
actual=$(checksum "$temporary")
if [ "$actual" != "$expected" ]; then
  echo "Forgejo source checksum mismatch: got $actual, expected $expected" >&2
  exit 1
fi
tar -tzf "$temporary" >/dev/null
mv "$temporary" "$output"
trap - 0 1 2 15
