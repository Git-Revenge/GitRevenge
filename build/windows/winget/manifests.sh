#!/usr/bin/env bash
# manifests.sh VERSION OUTDIR
# Writes the winget manifests for release vVERSION into OUTDIR, in the folder layout
# microsoft/winget-pkgs uses (manifests/g/GitRevenge/GitRevenge/VERSION). The installer's
# sha256 comes from the digest GitHub recorded for the release asset. Needs gh and jq.
set -euo pipefail
version=${1:?usage: manifests.sh VERSION OUTDIR}
out=${2:?usage: manifests.sh VERSION OUTDIR}
here=$(cd "$(dirname "$0")" && pwd)
asset="GitRevenge-$version-windows-amd64-installer.exe"

digest=$(gh api "repos/Git-Revenge/GitRevenge/releases/tags/v$version" \
  --jq ".assets[] | select(.name == \"$asset\") | .digest")
case "$digest" in
  sha256:*) sha=$(echo "${digest#sha256:}" | tr 'a-f' 'A-F') ;;
  *) echo "v$version has no $asset with a sha256 digest" >&2; exit 1 ;;
esac

dir="$out/manifests/g/GitRevenge/GitRevenge/$version"
mkdir -p "$dir"
for template in "$here"/GitRevenge.GitRevenge*.yaml; do
  sed -e "s/{{VERSION}}/$version/g" -e "s/{{SHA256}}/$sha/g" "$template" > "$dir/$(basename "$template")"
done
echo "$dir"
