#!/usr/bin/env bash
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
out=${1:?Output directory is required}
binary=${2:?Release executable is required}
target_os=${3:?Target OS is required}
target_arch=${4:?Target architecture is required}
[[ "$out" = /* ]] || out="$PWD/$out"
[[ "$binary" = /* ]] || binary="$PWD/$binary"
cd "$repo"
host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)
GOOS="$host_os" GOARCH="$host_arch" CGO_ENABLED=0 go run -mod=readonly ./scripts/licenses -binary "$binary" -out "$out" -os "$target_os" -arch "$target_arch"
