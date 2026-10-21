#!/usr/bin/env bash
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
caller_dir=$PWD
cd "$repo"
target_os=${1:?Target OS is required}
target_arch=${2:?Target architecture is required}
out=${3:?ZIP output directory is required}
[[ "$out" = /* ]] || out="$caller_dir/$out"
case "$target_os" in windows|darwin|linux) ;; *) echo 'Unsupported OS' >&2; exit 2;; esac
case "$target_arch" in amd64|arm64) ;; *) echo 'Unsupported architecture' >&2; exit 2;; esac
version=$(tr -d '\r\n' < VERSION)
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid VERSION' >&2; exit 1; }
[[ -s LICENSE ]] || { echo 'Finalize copyright holder and create LICENSE before packaging.' >&2; exit 1; }
if grep -Fq '{{COPYRIGHT_HOLDER}}' LICENSE; then
    echo 'LICENSE still contains an unresolved copyright holder.' >&2
    exit 1
fi
command -v zip >/dev/null || { echo 'zip is required; install it before packaging.' >&2; exit 1; }
mkdir -p -- "$out"
out=$(cd -- "$out" && pwd)
name="tque-$version-$target_os-$target_arch"
archive="$out/$name.zip"
[[ ! -e "$archive" ]] || { echo "Archive already exists: $archive" >&2; exit 1; }
work=$(mktemp -d "${TMPDIR:-/tmp}/tque-zip.XXXXXXXX")
stage="$work/$name"
mkdir -p -- "$stage"
bash scripts/build-release.sh "$target_os" "$target_arch" "$work/build"
suffix=''; [[ "$target_os" != windows ]] || suffix=.exe
cp -- "$work/build/tque-$target_os-$target_arch$suffix" "$stage/tque$suffix"
chmod 755 "$stage/tque$suffix"
cp LICENSE README.md README.ja.md CHANGELOG.md "$stage/"
cp -R docs "$stage/docs"
bash scripts/collect-licenses.sh "$stage/third-party-licenses"
(cd "$work" && zip -q -r "$archive" "$name")
(cd "$out" && if command -v sha256sum >/dev/null; then
    sha256sum "$name.zip" > "$name.zip.sha256"
else
    shasum -a 256 "$name.zip" > "$name.zip.sha256"
fi)
printf 'Packaged: %s\nStaging retained: %s\n' "$archive" "$work"
