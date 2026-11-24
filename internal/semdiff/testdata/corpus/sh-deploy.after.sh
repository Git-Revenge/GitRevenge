#!/usr/bin/env bash
set -euo pipefail
dest="build"
mkdir -p "$dest"
for f in *.go; do echo "building" "$f"; done
cp app "$dest/app"
echo "done in $dest"
cat <<EOF
  version: 2
EOF
