#!/bin/sh
# Скрипт полного удаления Sirin Router Client для OpenWrt
set -e

echo "=== Удаление Sirin Router Client ==="

# 1. Остановка и отключение службы
if [ -x /etc/init.d/sirin ]; then
    echo "Остановка службы sirin..."
    /etc/init.d/sirin stop 2>/dev/null || true
    /etc/init.d/sirin disable 2>/dev/null || true
fi

# 2. Удаление интерфейса и очистка маршрутов ядра
ip link del sirin0 2>/dev/null || true
ip route flush dev sirin0 2>/dev/null || true

# 3. Удаление бинарника, init-скрипта и конфигурации
echo "Удаление файлов клиента..."
rm -f /etc/init.d/sirin /usr/bin/sirin-client
rm -rf /etc/sirin

# 4. Удаление компонентов LuCI Web UI (если были установлены)
rm -f /usr/libexec/rpcd/sirin \
      /usr/share/rpcd/acl.d/luci-app-sirin.json \
      /usr/share/luci/menu.d/luci-app-sirin.json \
      /www/luci-static/resources/view/sirin/overview.js 2>/dev/null || true
rm -rf /www/luci-static/resources/view/sirin 2>/dev/null || true
rm -f /tmp/luci-indexcache /tmp/luci-modulecache/* 2>/dev/null || true
/etc/init.d/rpcd reload 2>/dev/null || true

# 5. Удаление sirin0 из firewall zone wan
if command -v uci >/dev/null 2>&1 && uci show firewall 2>/dev/null | grep -q "name='wan'"; then
    WAN_ZONE=$(uci show firewall 2>/dev/null | grep -E "\.name='wan'" | head -n1 | cut -d. -f2 | cut -d= -f1)
    [ -z "$WAN_ZONE" ] && WAN_ZONE="@zone[1]"

    if uci get "firewall.$WAN_ZONE.device" 2>/dev/null | grep -qw "sirin0"; then
        echo "Удаление sirin0 из зоны wan фаервола..."
        uci del_list "firewall.$WAN_ZONE.device=sirin0" 2>/dev/null || true
        uci commit firewall 2>/dev/null || true
        /etc/init.d/firewall reload 2>/dev/null || true
    fi
fi

echo ""
echo "Sirin удалён."
echo "Если настраивали strict-tunnel — откатите DNS вручную:"
echo "  uci revert dhcp; uci commit dhcp"
echo "  /etc/init.d/dnsmasq restart; /etc/init.d/odhcpd restart"
