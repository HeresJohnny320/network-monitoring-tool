#!/usr/bin/env bash
# Cross-compiles release archives for every supported platform into dist/.
# Used by the GitHub Actions workflow too, so local and CI builds are identical.
#
#   ./build.sh                 # version from git (e.g. v1.5.0 or v1.5.0-3-gabc123)
#   VERSION=v1.5.0 ./build.sh
#   ./build.sh linux/amd64     # build only some targets

set -euo pipefail

APP=network_monitor_tool
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}
DIST=dist

# os/arch[/arm version]. SQLite is pure Go (modernc.org/sqlite), so no C compiler is needed.
TARGETS=(
    linux/amd64
    linux/arm64
    linux/arm/7
    linux/arm/6
    linux/386
    windows/amd64
    windows/arm64
    darwin/amd64
    darwin/arm64
    freebsd/amd64
    freebsd/arm64
)
if [ $# -gt 0 ]; then
    TARGETS=("$@")
fi

rm -rf "$DIST"
mkdir -p "$DIST"

for target in "${TARGETS[@]}"; do
    IFS=/ read -r os arch arm <<<"$target"
    name="${os}_${arch}${arm:+v$arm}"
    stage="$DIST/stage/$name"
    bin="$APP"
    [ "$os" = windows ] && bin="$APP.exe"

    echo "Building $name ($VERSION)..."
    mkdir -p "$stage"
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch GOARM=$arm \
        go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$stage/$bin" .

    cp README.md LICENSE "$stage/"
    if [ "$os" = windows ]; then
        cp scripts/install.ps1 "$stage/"
        (cd "$stage" && zip -qr "../../${APP}_${name}.zip" .)
    else
        cp scripts/install.sh "$stage/"
        [ "$os" = freebsd ] && cp -r pfsense "$stage/"
        tar -czf "$DIST/${APP}_${name}.tar.gz" -C "$stage" .
    fi
done

rm -rf "$DIST/stage"
(cd "$DIST" && sha256sum $(ls *.tar.gz *.zip 2>/dev/null) > checksums.txt)
echo "Done! Archives are in $DIST/"
ls -lh "$DIST"
