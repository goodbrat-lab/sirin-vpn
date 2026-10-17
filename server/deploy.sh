#!/usr/bin/env bash
# deploy.sh — первичное развёртывание Sirin VPN на чистом Ubuntu 24.04
# Использование: sudo ./deploy.sh
set -euo pipefail

echo "=== Sirin VPN Deploy ==="
echo ""

# 1. Проверка root
if [ "$EUID" -ne 0 ]; then
  echo "ERROR: запускать от root (sudo ./deploy.sh)"
  exit 1
fi

# 2. Проверка Docker
if ! command -v docker &> /dev/null; then
  echo "Устанавливаю Docker..."
  apt update
  apt install -y ca-certificates curl gnupg
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu noble stable" > /etc/apt/sources.list.d/docker.list
  apt update
  apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi

# 3. Проверка прав docker compose
docker compose version || {
  echo "ERROR: docker compose plugin не установлен"
  exit 1
}

# 4. Проверка /dev/net/tun
if [ ! -c /dev/net/tun ]; then
  echo "ERROR: /dev/net/tun отсутствует. VPS должен поддерживать TUN."
  echo "На HostVDS — обычно есть по умолчанию."
  exit 1
fi

# 5. Проверка sysctl ip_forward
if [ "$(sysctl -n net.ipv4.ip_forward)" != "1" ]; then
  echo "Включаю ip_forward..."
  sysctl -w net.ipv4.ip_forward=1
  grep -q '^net.ipv4.ip_forward=1' /etc/sysctl.conf || echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
fi

# 6. Проверка конфигов
echo ""
echo "Проверка конфигов в /etc/sirin/:"
for f in sirind.toml sirinapi.toml sirinbot.toml static_priv.hex clients.json bot_users.json; do
  if [ ! -f "/etc/sirin/$f" ]; then
    echo "  MISSING: /etc/sirin/$f"
  else
    echo "  OK: /etc/sirin/$f"
  fi
done

if [ ! -f "/usr/local/etc/xray/config.json" ]; then
  echo "  MISSING: /usr/local/etc/xray/config.json"
  exit 1
fi

# 7. Проверка сертификата Let's Encrypt
if [ ! -d "/etc/letsencrypt/live/crp.guseynovia.ru" ]; then
  echo "ERROR: SSL-сертификат не найден."
  echo "Сначала: certbot certonly --nginx -d crp.guseynovia.ru"
  exit 1
fi

# 8. Сборка образа
cd "$(dirname "$0")"
echo ""
echo "Сборка sirin-server:latest..."
docker build --network host -t sirin-server:latest .

# 8.5. Права на /var/log/xray (xray в контейнере работает под UID 65532)
if [ -d "/var/log/xray" ]; then
  chown -R 65532:65532 /var/log/xray
  chmod 755 /var/log/xray
  echo "Fixed /var/log/xray permissions (UID 65532)"
fi

# 9. Запуск
echo ""
echo "Запуск docker compose..."
docker compose up -d

# 10. Проверка
sleep 3
echo ""
echo "Статус контейнеров:"
docker compose ps

echo ""
echo "=== Deploy complete ==="
echo "Проверка: docker compose logs -f"
