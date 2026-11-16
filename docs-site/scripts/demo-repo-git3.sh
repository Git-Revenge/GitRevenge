#!/usr/bin/env bash
# Builds Tidewater the way Git 3.0 will make a new repository: SHA-256 object IDs, reftable
# refs and a main branch, with git refusing any bare repository it isn't pointed at.
#
#   docs-site/scripts/demo-repo-git3.sh <folder>
#
# It makes <folder>/tidewater-git3, <folder>/origin-git3.git and the rest, with the same
# history as demo-repo.sh, so the two can sit side by side. It needs git 2.45 or later, and
# demo-repo.sh in the same folder.
set -euo pipefail
exec bash "$(cd "$(dirname "$0")" && pwd)/demo-repo.sh" --git3 "$@"
