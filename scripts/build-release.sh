#!/usr/bin/env bash
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
caller_dir=$PWD
cd "$repo"
target_os=${1:-$(go env GOOS)}
target_arch=${2:-$(go env GOARCH)}
out=${3:-"$repo/dist"}
[[ "$out" = /* ]] || out="$caller_dir/$out"
case "$target_os" in windows|darwin|linux) ;; *) echo 'Unsupported OS' >&2; exit 2;; esac
case "$target_arch" in amd64|arm64) ;; *) echo 'Unsupported architecture' >&2; exit 2;; esac
version=$(tr -d '\r\n' < VERSION)
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid VERSION' >&2; exit 2; }
mkdir -p -- "$out"
suffix=''; [[ "$target_os" != windows ]] || suffix=.exe
GOOS="$target_os" GOARCH="$target_arch" CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags '-s -w' -o "$out/tque-$target_os-$target_arch$suffix" .
printf 'Built tque %s (%s/%s)\n' "$version" "$target_os" "$target_arch"
