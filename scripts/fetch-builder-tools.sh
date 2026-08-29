#!/bin/sh
set -eu

version=go1.27.0

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
  architecture=$1
  expected=$2
  output=".artifacts/tools/${version}.linux-${architecture}.tar.gz"
  if [ -f "$output" ] && [ "$(checksum "$output")" = "$expected" ]; then
    return
  fi
  mkdir -p "$(dirname "$output")"
  curl --fail --location --output "$output.tmp" "https://go.dev/dl/${version}.linux-${architecture}.tar.gz"
  actual=$(checksum "$output.tmp")
  if [ "$actual" != "$expected" ]; then
    echo "Go toolchain checksum mismatch for linux/$architecture: got $actual, expected $expected" >&2
    exit 1
  fi
  mv "$output.tmp" "$output"
}

fetch_toolchain amd64 675c26c449cbb18fc24b74650de1eabbae6e16f64326fd85a283fb3b58280685
fetch_toolchain arm64 51798d2c42d0e1c6ed7fd9f48728b4193abac9e8aad6dbac2fe96a81f5909bda
