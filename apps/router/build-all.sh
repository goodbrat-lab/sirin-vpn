#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"
ROOT_DIR="$(pwd)"

OUT=dist
mkdir -p "$OUT"

VERSION=${VERSION:-1.1.0}
LDFLAGS="-s -w -X main.Version=$VERSION"

echo "Building sirin-client v$VERSION for 5 architectures..."

# MIPS (big-endian)
CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat \
    go build -ldflags="$LDFLAGS" -o "$OUT/sirin-client-mips" .

# MIPSLE (little-endian, самый популярный для роутеров)
CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat \
    go build -ldflags="$LDFLAGS" -o "$OUT/sirin-client-mipsle" .

# ARMv7
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
    go build -ldflags="$LDFLAGS" -o "$OUT/sirin-client-armv7" .

# ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -ldflags="$LDFLAGS" -o "$OUT/sirin-client-arm64" .

# x86_64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="$LDFLAGS" -o "$OUT/sirin-client-x86_64" .

echo ""
echo "Sizes:"
ls -lh "$OUT"/sirin-client-* | grep -v '\.tar\.gz' | awk '{print $9, $5}'

# Упаковать в tar.gz (включаем client.toml.example как в корне архива, так и в configs/)
for bin in "$OUT"/sirin-client-*; do
    [ -f "$bin" ] || continue
    case "$bin" in
        *.tar.gz) continue ;;
    esac
    arch=$(basename "$bin" | sed 's/sirin-client-//')
    tar -czf "$OUT/sirin-client-$VERSION-$arch.tar.gz" \
        -C "$OUT" "sirin-client-$arch" \
        -C "$ROOT_DIR" "init.d/sirin" \
        -C "$ROOT_DIR/configs" "client.toml.example" \
        -C "$ROOT_DIR" "configs/client.toml.example" \
        -C "$ROOT_DIR" "luci-app-sirin"
done

echo ""
echo "Archives:"
ls -lh "$OUT"/*.tar.gz
