#!/usr/bin/env bash
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
tag=${1:?Release tag is required}
asset_dir=${2:?Asset directory is required}
version=$(tr -d '\r\n' < "$repo/VERSION")
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ && "$tag" == "v$version" ]] || {
    echo 'Release tag must equal v followed by VERSION.' >&2; exit 1;
}
asset_dir=$(cd -- "$asset_dir" && pwd)
assets=()
for target_os in windows darwin linux; do
    for target_arch in amd64 arm64; do
        name="tque-$version-$target_os-$target_arch.zip"
        [[ -s "$asset_dir/$name" && -s "$asset_dir/$name.sha256" ]] || {
            echo "Missing or empty release asset: $name or its checksum" >&2; exit 1;
        }
        # Check the expected filename as well as the digest; a checksum naming
        # another file must not substitute for validation of this archive.
        actual=$(cd "$asset_dir" && sha256sum "$name")
        recorded=$(cat "$asset_dir/$name.sha256")
        [[ "$actual" == "$recorded" ]] || { echo "Checksum mismatch: $name" >&2; exit 1; }
        unzip -tq "$asset_dir/$name"
        assets+=("$asset_dir/$name" "$asset_dir/$name.sha256")
    done
done
: "${GH_REPO:?GitHub repository must be supplied in GH_REPO}"
# gh refuses an existing release. Do not replace published assets on reruns.
# --verify-tag prevents gh from creating a tag at a different commit.
gh release create "$tag" "${assets[@]}" --repo "$GH_REPO" \
    --verify-tag --title "tque $tag" --generate-notes
