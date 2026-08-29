#!/bin/sh
set -eu

case "$(uname -s):$(uname -m)" in
  Darwin:x86_64) asset=cosign-darwin-amd64; checksum=acd180f8b015be25240ca33abee8a1e564eb65cdf1a3cee4725456d2dceb7da6 ;;
  Darwin:arm64) asset=cosign-darwin-arm64; checksum=dec1c3f802320b19c2fbcf2dc7bcfb3f258e1c181a046c23a1a074bdf932f10a ;;
  Linux:x86_64) asset=cosign-linux-amd64; checksum=f7622ed3cf22e55e1ae6377c080979ff77a22da9981c11df222a2e444991e7cf ;;
  Linux:aarch64) asset=cosign-linux-arm64; checksum=90e7ae0b5dfd60f20816b52c012addf7fc055ebcc7bea4ce81c428ca8518c302 ;;
  *) echo "unsupported release host $(uname -s)/$(uname -m)" >&2; exit 1 ;;
esac

fetch_cosign() {
  fetch_asset=$1
  fetch_checksum=$2
  fetch_output=$3
  mkdir -p "$(dirname "$fetch_output")"
  curl --fail --location --output "$fetch_output.tmp" "https://github.com/sigstore/cosign/releases/download/v3.1.2/$fetch_asset"
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$fetch_output.tmp" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$fetch_output.tmp" | awk '{print $1}')
  else
    echo "a SHA-256 checksum utility is required" >&2
    exit 1
  fi
  if [ "$actual" != "$fetch_checksum" ]; then
    echo "cosign checksum mismatch for $fetch_asset: got $actual, expected $fetch_checksum" >&2
    exit 1
  fi
  chmod 0755 "$fetch_output.tmp"
  mv "$fetch_output.tmp" "$fetch_output"
}

host_output=.artifacts/tools/cosign
fetch_cosign "$asset" "$checksum" "$host_output"
for target in arm64 amd64; do
  case "$target" in
    arm64) target_checksum=90e7ae0b5dfd60f20816b52c012addf7fc055ebcc7bea4ce81c428ca8518c302 ;;
    amd64) target_checksum=f7622ed3cf22e55e1ae6377c080979ff77a22da9981c11df222a2e444991e7cf ;;
  esac
  target_asset=cosign-linux-$target
  target_output=.artifacts/tools/$target_asset
  if [ "$asset" = "$target_asset" ]; then
    cp "$host_output" "$target_output"
  else
    fetch_cosign "$target_asset" "$target_checksum" "$target_output"
  fi
done
"$host_output" version
