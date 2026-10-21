#!/usr/bin/env bash
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
out=${1:?Output directory is required}
mkdir -p -- "$out"
out=$(cd -- "$out" && pwd)
cd "$repo"
go mod download all
modules=$(go list -m -f '{{.Path}}|{{.Version}}|{{.Dir}}' all)
: > "$out/MODULES.txt"
while IFS='|' read -r module version directory; do
    [[ "$module" != tque ]] || continue
    name=$(printf '%s@%s' "$module" "$version" | tr '/:' '__')
    mkdir -p -- "$out/$name"
    found=0
    while IFS= read -r -d '' file; do
        relative=${file#"$directory/"}
        destination="$out/$name/$relative"
        mkdir -p -- "$(dirname -- "$destination")"
        cp -- "$file" "$destination"
        found=1
    done < <(find "$directory" -type f \( -iname 'LICENSE*' -o -iname 'COPYING*' -o -iname 'NOTICE*' -o -iname 'COPYRIGHT*' \) -print0)
    [[ "$found" == 1 ]] || { printf 'Missing license: %s\n' "$module" >&2; exit 1; }
    printf '%s %s\n' "$module" "$version" >> "$out/MODULES.txt"
done <<< "$modules"
cp -- "$(go env GOROOT)/LICENSE" "$out/GO-LICENSE"
