#!/bin/sh
# Универсальный установщик для OpenWrt 24.x (opkg) и 25.x (apk)
set -e

UI_MODE="auto"
ARCH=""
for arg in "$@"; do
    case "$arg" in
        --with-ui) UI_MODE="with" ;;
        --no-ui)   UI_MODE="no" ;;
        mips|mipsle|armv7|arm64|x86_64) ARCH="$arg" ;;
    esac
done

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

VERSION=${VERSION:-1.1.1}
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
mkdir -p /usr/bin /etc/sirin /etc/init.d || true

BIN_SRC=""
for p in "/tmp/sirin-client-$ARCH" "/tmp/sirin-client" "/tmp/dist/sirin-client-$ARCH"; do
    if [ -f "$p" ]; then
        BIN_SRC="$p"
        break
    fi
done
if [ -z "$BIN_SRC" ]; then
    BIN_SRC=$(find /tmp -name "sirin-client*" -type f ! -name "*.tar.gz" 2>/dev/null | head -n1)
fi

if [ -n "$BIN_SRC" ]; then
    cp "$BIN_SRC" /usr/bin/sirin-client
    chmod +x /usr/bin/sirin-client
    echo "Binary: /usr/bin/sirin-client (из $BIN_SRC)"
else
    echo "ERROR: бинарник sirin-client-$ARCH не найден в распакованном архиве"
    exit 1
fi

# 3. Конфиг (сохраняем существующий при обновлении)
if [ ! -f /etc/sirin/client.toml ]; then
    CONF_SRC=""
    for p in /tmp/client.toml.example /tmp/configs/client.toml.example /tmp/client.toml.template /tmp/configs/client.toml.template; do
        if [ -f "$p" ]; then
            CONF_SRC="$p"
            break
        fi
    done
    if [ -z "$CONF_SRC" ]; then
        CONF_SRC=$(find /tmp -name "client.toml*" -type f 2>/dev/null | head -n1)
    fi

    if [ -n "$CONF_SRC" ]; then
        cp "$CONF_SRC" /etc/sirin/client.toml
        echo "Config: /etc/sirin/client.toml (из $CONF_SRC)"
    else
        echo "WARNING: шаблон client.toml не найден, создайте /etc/sirin/client.toml вручную"
    fi
fi

# 4. Init-скрипт
INIT_SRC=""
for p in /tmp/init.d/sirin /tmp/sirin; do
    if [ -f "$p" ]; then
        INIT_SRC="$p"
        break
    fi
done
if [ -z "$INIT_SRC" ]; then
    INIT_SRC=$(find /tmp -name "sirin" -type f ! -name "*.tar.gz" ! -name "sirin-client*" 2>/dev/null | head -n1)
fi

if [ -n "$INIT_SRC" ]; then
    cp "$INIT_SRC" /etc/init.d/sirin
    chmod +x /etc/init.d/sirin
    echo "Service: /etc/init.d/sirin (из $INIT_SRC)"
else
    echo "WARNING: init-скрипт sirin не найден"
fi

# 5. Веб-интерфейс LuCI
HAS_LUCI=0
if [ -d /www/luci-static/resources ] && [ -d /usr/share/luci ]; then
    HAS_LUCI=1
fi

INSTALL_UI=0
case "$UI_MODE" in
    with)
        INSTALL_UI=1
        ;;
    no)
        INSTALL_UI=0
        ;;
    auto)
        if [ "$HAS_LUCI" = 1 ]; then
            INSTALL_UI=1
        fi
        ;;
esac

UI_INSTALLED=0
if [ "$INSTALL_UI" = 1 ]; then
    LUCI_DIR=""
    for p in /tmp/luci-app-sirin /tmp/*/luci-app-sirin; do
        if [ -d "$p" ]; then
            LUCI_DIR="$p"
            break
        fi
    done

    if [ -n "$LUCI_DIR" ]; then
        echo "Installing LuCI Web UI..."
        if [ -d "$LUCI_DIR/root" ]; then
            cp -r "$LUCI_DIR/root/"* / 2>/dev/null || true
        fi
        if [ -d "$LUCI_DIR/htdocs" ]; then
            mkdir -p /www
            cp -r "$LUCI_DIR/htdocs/"* /www/ 2>/dev/null || true
        fi
        chmod +x /usr/libexec/rpcd/sirin 2>/dev/null || true
        /etc/init.d/rpcd reload 2>/dev/null || /etc/init.d/rpcd restart 2>/dev/null || true
        rm -f /tmp/luci-indexcache /tmp/luci-modulecache/* 2>/dev/null || true
        UI_INSTALLED=1
        echo "UI: http://<router-ip>/cgi-bin/luci/admin/services/sirin"
    else
        echo "WARNING: каталог luci-app-sirin не найден в архиве"
    fi
else
    echo "UI: пропущен (LuCI не обнаружен или --no-ui)"
fi

# 6. Зависимости (kmod-tun + nftables)
echo "Installing dependencies..."
if [ "$PKG" = "apk" ]; then
    apk update 2>/dev/null || true
    apk add kmod-tun nftables ip-full wget-ssl 2>/dev/null || \
    apk add kmod-tun nftables ip-full 2>/dev/null || true
else
    opkg update 2>/dev/null || true
    opkg install kmod-tun nftables ip-full wget-ssl 2>/dev/null || \
    opkg install kmod-tun nftables ip-full 2>/dev/null || true
fi

# 7. Проверить, что TUN доступен
if [ ! -c /dev/net/tun ]; then
    echo "WARNING: /dev/net/tun не найден. Установите kmod-tun:"
    echo "  $PKG_INSTALL kmod-tun"
fi

# 8. Включить автозапуск
/etc/init.d/sirin enable 2>/dev/null || true

echo ""
echo "=== Sirin Router Client установлен ==="
echo "Конфигурация: /etc/sirin/client.toml"
if [ "$UI_INSTALLED" = 1 ]; then
    echo "Веб-интерфейс LuCI: http://<router-ip>/cgi-bin/luci/admin/services/sirin"
fi
echo "1. Отредактируйте конфиг и вставьте ваш UUID из @sirin_crp_bot:"
echo "   vi /etc/sirin/client.toml"
echo "2. Запустите службу:"
echo "   /etc/init.d/sirin start"
echo "3. Проверьте журнал работы:"
echo "   logread -f | grep sirin"
