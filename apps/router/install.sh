#!/bin/sh
# Универсальный установщик для OpenWrt 24.x (opkg) и 25.x (apk)
set -e

ARCH=${1:-}
if [ -z "$ARCH" ]; then
    echo "Detecting architecture..."
    M=$(uname -m)
    case "$M" in
        mips)      ARCH=mips ;;
        mipsel)    ARCH=mipsle ;;
        armv7l)    ARCH=armv7 ;;
        aarch64)   ARCH=arm64 ;;
        x86_64)    ARCH=x86_64 ;;
        *)         echo "Unknown arch: $M"; exit 1 ;;
    esac
    echo "Detected: $ARCH"
fi

VERSION=${VERSION:-1.0.0}
URL="https://github.com/goodbrat-lab/sirin-vpn/releases/download/router-v$VERSION/sirin-client-$VERSION-$ARCH.tar.gz"

# Определить пакетный менеджер
if command -v apk >/dev/null 2>&1; then
    PKG=apk
    PKG_INSTALL="apk add"
elif command -v opkg >/dev/null 2>&1; then
    PKG=opkg
    PKG_INSTALL="opkg install"
else
    echo "ERROR: ни apk, ни opkg не найдены"
    exit 1
fi
echo "Package manager: $PKG"

# 1. Скачать
echo "Downloading $URL..."
wget -O /tmp/sirin.tar.gz "$URL"
tar -xzf /tmp/sirin.tar.gz -C /tmp/

# 2. Установить бинарник
mkdir -p /usr/bin /etc/sirin /etc/init.d
cp /tmp/sirin-client-$ARCH /usr/bin/sirin-client
chmod +x /usr/bin/sirin-client

# 3. Конфиг
if [ ! -f /etc/sirin/client.toml ]; then
    cp /tmp/client.toml.example /etc/sirin/client.toml
    echo ""
    echo "WARNING: edit /etc/sirin/client.toml and set password (UUID from @sirin_crp_bot)"
fi

# 4. Init-скрипт
cp /tmp/init.d/sirin /etc/init.d/sirin
chmod +x /etc/init.d/sirin

# 5. Зависимости (kmod-tun + nftables)
echo "Installing dependencies..."
if [ "$PKG" = "apk" ]; then
    apk update
    apk add kmod-tun nftables ip-full wget-ssl 2>/dev/null || \
    apk add kmod-tun nftables ip-full 2>/dev/null || true
else
    opkg update
    opkg install kmod-tun nftables ip-full wget-ssl 2>/dev/null || \
    opkg install kmod-tun nftables ip-full 2>/dev/null || true
fi

# 6. Проверить, что TUN доступен
if [ ! -c /dev/net/tun ]; then
    echo "WARNING: /dev/net/tun не найден. Установи kmod-tun:"
    echo "  $PKG_INSTALL kmod-tun"
fi

# 7. Включить автозапуск
/etc/init.d/sirin enable
echo ""
echo "=== Установлено ==="
echo "1. Отредактируй /etc/sirin/client.toml (вставь UUID)"
echo "2. Запусти: /etc/init.d/sirin start"
echo "3. Проверь: logread -f | grep sirin"
