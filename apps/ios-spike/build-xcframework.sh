#!/bin/bash
set -e

# build-xcframework.sh — сборка mobile-модуля Sirin под iOS (arm64 + simulator)
# Использование:
#   cd apps/ios-spike
#   ./build-xcframework.sh

echo "=== 1. Проверка окружения ==="
command -v go >/dev/null 2>&1 || { echo "Go не найден"; exit 1; }
command -v gomobile >/dev/null 2>&1 || {
    echo "Установка gomobile..."
    go install golang.org/x/mobile/cmd/gomobile@latest
    go install golang.org/x/mobile/cmd/gobind@latest
    gomobile init
}
command -v gobind >/dev/null 2>&1 || {
    go install golang.org/x/mobile/cmd/gobind@latest
}

cd "$(dirname "$0")/mobile"
go get golang.org/x/mobile/bind
go mod tidy

echo "=== 2. Сборка Mobile.xcframework через gomobile bind ==="
# Флаги -s -w вырезают отладочные символы для экономии размера
# Флаг -ldflags="-checklinkname=0" страхует от проверок linkname в pion/turn
gomobile bind \
    -target=ios,iossimulator \
    -ldflags="-s -w -checklinkname=0" \
    -o ../Mobile.xcframework \
    .

echo "=== 3. Проверка размера артефакта ==="
ls -lh ../Mobile.xcframework
echo "Готово: apps/ios-spike/Mobile.xcframework собран успешно!"
