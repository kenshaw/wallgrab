#!/bin/bash

SRC="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ua=$(verhist -latest-ua)

(set -x;
  curl \
    -H "User-Agent: $ua" \
    -o $SRC/apple_ca_bundle.pem \
    https://api.tlsinspector.com/rootca/asset/latest/apple_ca_bundle.pem
)
