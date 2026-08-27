#!/bin/sh
set -eu

case "$(uname -s):$(uname -m)" in
  Darwin:x86_64) asset=cosign-darwin-amd64; checksum=acd180f8b015be25240ca33abee8a1e564eb65cdf1a3cee4725456d2dceb7da6 ;;
  Darwin:arm64) asset=cosign-darwin-arm64; checksum=dec1c3f802320b19c2fbcf2dc7bcfb3f258e1c181a046c23a1a074bdf932f10a ;;
  Linux:x86_64) asset=cosign-linux-amd64; checksum=f7622ed3cf22e55e1ae6377c080979ff77a22da9981c11df222a2e444991e7cf ;;
  Linux:aarch64) asset=cosign-linux-arm64; checksum=90e7ae0b5dfd60f20816b52c012addf7fc055ebcc7bea4ce81c428ca8518c302 ;;
  *) echo "unsupported release host $(uname -s)/$(uname -m)" >&2; exit 1 ;;
esac

output=.artifacts/tools/cosign
mkdir -p "$(dirname "$output")"
curl --fail --location --output "$output.tmp" "https://github.com/sigstore/cosign/releases/download/v3.1.2/$asset"
actual=$(shasum -a 256 "$output.tmp" | awk '{print $1}')
if [ "$actual" != "$checksum" ]; then
  echo "cosign checksum mismatch: got $actual, expected $checksum" >&2
  exit 1
fi
chmod 0755 "$output.tmp"
mv "$output.tmp" "$output"
"$output" version
