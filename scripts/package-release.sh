#!/usr/bin/env bash
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo"
target_os=${1:-$(go env GOOS)}
target_arch=${2:-$(go env GOARCH)}
case "$target_os" in darwin|linux) ;; *) echo 'Use build-installer.ps1 for Windows' >&2; exit 2;; esac
[[ -f LICENSE ]] || { echo 'Finalize copyright holder and create LICENSE before packaging.' >&2; exit 1; }
if grep -Fq '{{COPYRIGHT_HOLDER}}' LICENSE; then echo 'LICENSE still contains an unresolved copyright holder.' >&2; exit 1; fi
bash scripts/build-release.sh "$target_os" "$target_arch"
version=$(tr -d '\r\n' < VERSION)
mkdir -p dist
stage=$(mktemp -d "$repo/dist/package.XXXXXXXX")
# Keep staging on failure for inspection; never remove a computed directory recursively.
cp "dist/tque-$target_os-$target_arch" "$stage/tque"
chmod 755 "$stage/tque"
cp LICENSE README.md README.ja.md CHANGELOG.md "$stage/"
mkdir -p "$stage/docs"
cp docs/user-manual.md docs/user-manual.ja.md "$stage/docs/"
bash scripts/collect-licenses.sh "$stage/third-party-licenses" "$stage/tque" "$target_os" "$target_arch"
archive="$repo/dist/tque-$version-$target_os-$target_arch.tar.gz"
tar -czf "$archive" -C "$stage" .
if command -v sha256sum >/dev/null; then
    (cd dist && sha256sum "$(basename "$archive")" > "$(basename "$archive").sha256")
else
    (cd dist && shasum -a 256 "$(basename "$archive")" > "$(basename "$archive").sha256")
fi
printf 'Packaged: %s\nStaging retained: %s\n' "$archive" "$stage"
