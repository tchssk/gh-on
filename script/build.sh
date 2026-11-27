#!/bin/bash
# Builds the release binaries for cli/gh-extension-precompile.
# Windows is not supported because rule commands run with /bin/sh.
set -euo pipefail

tag="$1"
platforms=(
  darwin-amd64
  darwin-arm64
  freebsd-386
  freebsd-amd64
  freebsd-arm64
  linux-386
  linux-amd64
  linux-arm
  linux-arm64
)

for p in "${platforms[@]}"; do
  GOOS="${p%-*}" GOARCH="${p#*-}" CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X=github.com/tchssk/gh-on/cmd.Version=${tag#v}" \
    -o "dist/${p}" .
done
