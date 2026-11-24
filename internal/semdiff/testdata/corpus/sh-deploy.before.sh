#!/usr/bin/env bash
set -euo pipefail
out="build"
mkdir -p "$out"
for f in *.go; do echo 'building'  "$f"; done
cp app "$out/app"; echo "done in $out"
cat <<EOF
  version: 1
EOF
