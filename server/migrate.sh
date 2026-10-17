#!/usr/bin/env bash
# migrate.sh — миграция с systemd на Docker без простоя
# Использование: sudo ./migrate.sh
set -euo pipefail

echo "=== Sirin VPN: миграция systemd → Docker ==="
echo "ВАЖНО: этот скрипт остановит systemd-сервисы."
read -p "Продолжить? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Отменено."
  exit 1
fi

cd "$(dirname "$0")"

# 1. Остановить systemd-сервисы
echo ""
echo "Остановка systemd-сервисов..."
systemctl stop nginx xray sirind sirinapi sirinbot 2>/dev/null || true
systemctl disable nginx xray sirind sirinapi sirinbot 2>/dev/null || true

# 2. Собрать образ
echo ""
echo "Сборка sirin-server:latest..."
docker build -t sirin-server:latest .

# 3. Запустить compose
echo ""
echo "Запуск docker compose..."
docker compose up -d

# 4. Проверить
sleep 3
echo ""
echo "Статус:"
docker compose ps

# 5. Smoke-тест
echo ""
echo "Smoke-тест:"
curl -sf https://crp.guseynovia.ru/health && echo " API OK" || echo " API FAIL"
ss -ulnp | grep 56005 && echo " UDP OK" || echo " UDP FAIL"
ss -tlnp | grep 443 && echo " TCP 443 OK" || echo " TCP 443 FAIL"

echo ""
echo "=== Migration complete ==="
echo "Откат: sudo systemctl start nginx xray sirind sirinapi sirinbot && docker compose down"
