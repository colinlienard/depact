#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

VERSION=$(node -p "require('./npm/cli/package.json').version")

node -e '
  const fs = require("fs");
  const p = "npm/cli/package.json";
  const j = JSON.parse(fs.readFileSync(p));
  for (const k in (j.optionalDependencies || {})) j.optionalDependencies[k] = process.argv[1];
  fs.writeFileSync(p, JSON.stringify(j, null, 2) + "\n");
' "$VERSION"

# Go GOOS  Go GOARCH  npm os   npm cpu  package-name os  binary
targets=(
  "darwin  arm64  darwin  arm64  darwin   depact"
  "darwin  amd64  darwin  x64    darwin   depact"
  "linux   amd64  linux   x64    linux    depact"
  "linux   arm64  linux   arm64  linux    depact"
  "windows amd64  win32   x64    windows  depact.exe"
)

rm -rf npm/cli-darwin-* npm/cli-linux-* npm/cli-windows-*

for t in "${targets[@]}"; do
  read -r GOOS GOARCH NODE_OS NODE_CPU PKG_OS BIN <<<"$t"
  pkg="npm/cli-${PKG_OS}-${NODE_CPU}"

  echo "building ${GOOS}/${GOARCH} -> ${pkg}"
  mkdir -p "$pkg/bin"

  GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w" -o "$pkg/bin/$BIN" .

  export VERSION NODE_OS NODE_CPU PKG_OS
  envsubst <npm/package.json.tmpl >"$pkg/package.json"
done

echo "platform packages written under npm/"
