#!/bin/bash

SRC="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

REPO=tls-inspector/rootca

set -e

ua=$(verhist -latest-ua)

TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

tag=$(curl -sSfL \
  -H "User-Agent: $ua" \
  https://api.github.com/repos/$REPO/releases/latest \
  |jq -r '.tag_name')

echo "latest: $tag"

(set -x;
  curl -sSfL \
    -H "User-Agent: $ua" \
    --output-dir $TMP \
    -O https://github.com/$REPO/releases/download/$tag/apple_ca_bundle.pem \
    -O https://github.com/$REPO/releases/download/$tag/apple_ca_bundle.pem.sig \
    -O https://github.com/$REPO/releases/download/$tag/signing_key.pem
)

# the bundle is signed by the project's ecdsa key, which is pinned here so that
# a swapped key in the release is not silently trusted
pinned=10fc0e61bfd7e7b079ee4bbea8aa90aeb26b801640cae997c9f0c8f87b033301
actual=$(openssl dgst -sha256 -r $TMP/signing_key.pem|awk '{print $1}')
if [ "$pinned" != "$actual" ]; then
  echo "error: signing_key.pem does not match the pinned key ($actual)" >&2
  exit 1
fi

openssl dgst -sha256 \
  -verify $TMP/signing_key.pem \
  -signature $TMP/apple_ca_bundle.pem.sig \
  $TMP/apple_ca_bundle.pem

cp $TMP/apple_ca_bundle.pem $SRC/apple_ca_bundle.pem
