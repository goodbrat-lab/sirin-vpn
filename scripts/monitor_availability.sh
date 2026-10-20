#!/usr/bin/env bash
# ==============================================================================
# Sirin VPN — Скрипт синтетического мониторинга доступности из РФ
# Назначение: каждые 5 минут проверять доступность эндпоинта /health
# Порог: 3 последовательные ошибки соединения -> отправка алерта в Telegram
#
# Установка в cron:
#   */5 * * * * /usr/local/bin/monitor_availability.sh >> /var/log/sirin/monitor.log 2>&1
# ==============================================================================

set -u

# --- Конфигурация по умолчанию (переопределяется переменными окружения или аргументом) ---
API_DOMAIN="${API_DOMAIN:-crp.guseynovia.ru}"
PORT="${PORT:-443}"
TIMEOUT_SEC="${TIMEOUT_SEC:-5}"
FAIL_THRESHOLD="${FAIL_THRESHOLD:-3}"

# Загрузка переменных окружения из защищенного файла
ENV_FILE="/etc/sirin/monitor.env"
if [ -f "$ENV_FILE" ]; then
    # shellcheck disable=SC1090
    . "$ENV_FILE"
fi

# IP шлюза: аргумент $1 -> переменная VPN_NODE_IP -> авто-резолв API_DOMAIN
VPN_TARGET_IP="${1:-${VPN_NODE_IP:-}}"
if [ -z "$VPN_TARGET_IP" ]; then
    VPN_TARGET_IP=$(getent ahosts "$API_DOMAIN" 2>/dev/null | awk '{print $1}' | head -n 1)
fi

TG_AUTH="${TG_BOT_TOKEN:-${TELEGRAM_BOT_TOKEN:-}}"
TG_RECIPIENT="${TG_CHAT_ID:-${TELEGRAM_CHAT_ID:-}}"

# Файлы состояния
STATE_DIR="/tmp"
FAIL_FILE="${STATE_DIR}/sirin_monitor_fail_count"
ALERT_SENT_FILE="${STATE_DIR}/sirin_monitor_alert_sent"

# Функция отправки уведомления в Telegram
send_telegram() {
    local text="$1"
    if [ -z "$TG_AUTH" ] || [ -z "$TG_RECIPIENT" ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] [WARN] Telegram credentials not configured. Skipping alert."
        return 0
    fi

    curl -s -m 10 -X POST "https://api.telegram.org/bot${TG_AUTH}/sendMessage" \
        -d "chat_id=${TG_RECIPIENT}" \
        -d "text=${text}" \
        -d "parse_mode=Markdown" > /dev/null 2>&1 || true
}

# Текущее количество неудач
FAIL_COUNT=0
if [ -f "$FAIL_FILE" ]; then
    FAIL_COUNT=$(cat "$FAIL_FILE" 2>/dev/null || echo 0)
fi

# Проверка доступности через curl с принудительным сопоставлением домена и IP
URL="https://${API_DOMAIN}:${PORT}/health"
if [ -n "$VPN_TARGET_IP" ]; then
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -m "${TIMEOUT_SEC}" \
        --resolve "${API_DOMAIN}:${PORT}:${VPN_TARGET_IP}" \
        "$URL" 2>/dev/null)
else
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -m "${TIMEOUT_SEC}" "$URL" 2>/dev/null)
fi

if [ -z "$HTTP_CODE" ]; then
    HTTP_CODE="000"
fi

TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

if [ "$HTTP_CODE" = "200" ]; then
    # Успешный опрос
    echo "[$TIMESTAMP] [OK] Health check passed (HTTP 200) for target: ${VPN_TARGET_IP:-$API_DOMAIN}"
    if [ "$FAIL_COUNT" -ge "$FAIL_THRESHOLD" ] || [ -f "$ALERT_SENT_FILE" ]; then
        MSG="✅ *Sirin VPN: Доступность восстановлена*
*Хост:* \`${VPN_TARGET_IP:-$API_DOMAIN}\` (\`${API_DOMAIN}\`)
*Статус:* HTTP 200 OK
*Время:* ${TIMESTAMP}"
        send_telegram "$MSG"
        rm -f "$ALERT_SENT_FILE"
    fi
    echo 0 > "$FAIL_FILE"
    exit 0
else
    # Ошибка опроса
    FAIL_COUNT=$((FAIL_COUNT + 1))
    echo "$FAIL_COUNT" > "$FAIL_FILE"
    echo "[$TIMESTAMP] [FAIL] Health check failed for target: ${VPN_TARGET_IP:-$API_DOMAIN}, code: $HTTP_CODE, fail streak: $FAIL_COUNT"

    if [ "$FAIL_COUNT" -ge "$FAIL_THRESHOLD" ] && [ ! -f "$ALERT_SENT_FILE" ]; then
        MSG="🚨 *ВНИМАНИЕ: Сбой доступности Sirin VPN из РФ!*
*Узел:* \`${VPN_TARGET_IP:-$API_DOMAIN}\` (\`${API_DOMAIN}\`)
*Ошибка:* HTTP ${HTTP_CODE} (Таймаут или блокировка ТСПУ/РКН)
*Неудачных попыток подряд:* ${FAIL_COUNT}
*Время детекции:* ${TIMESTAMP}

_Возможна блокировка IP-адреса шлюза или сбой демона sirind/nginx._"
        send_telegram "$MSG"
        touch "$ALERT_SENT_FILE"
    fi
    exit 1
fi
