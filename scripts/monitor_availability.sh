#!/usr/bin/env bash
# ==============================================================================
# Sirin VPN — Probe-нода: Синтетический мониторинг доступности из РФ
# Назначение: каждые 5 минут проверять доступность ДВУХ эндпоинтов:
#   1. VPN: Gateway host port 443 (TCP connect / HTTPS health)
#   2. API: https://crp.guseynovia.ru/health (HTTP GET)
# Порог: 3 последовательные ошибки -> Telegram алерт
# Логирование: /var/log/sirin-probe.log (logrotate)
# ==============================================================================

set -u

CONFIG_FILE="/etc/sirin-probe/config.env"
if [ -f "$CONFIG_FILE" ]; then
    # shellcheck disable=SC1090
    . "$CONFIG_FILE"
fi

VPN_HOST="${VPN_HOST:-${1:-}}"
if [ -z "$VPN_HOST" ]; then
    VPN_HOST=$(getent ahosts "crp.guseynovia.ru" 2>/dev/null | awk '{print $1}' | head -n 1)
fi
VPN_PORT="${VPN_PORT:-443}"
VPN_HEALTH_URL="${VPN_HEALTH_URL:-https://${VPN_HOST}/health}"
API_URL="${API_URL:-https://crp.guseynovia.ru/health}"
TIMEOUT_SEC="${TIMEOUT_SEC:-5}"
FAIL_THRESHOLD="${FAIL_THRESHOLD:-3}"
LOG_FILE="${LOG_FILE:-/var/log/sirin-probe.log}"
STATE_DIR="${STATE_DIR:-/var/lib/sirin-probe}"

TG_AUTH="${TG_BOT_TOKEN:-${TELEGRAM_BOT_TOKEN:-}}"
TG_RECIPIENT="${TG_CHAT_ID:-${TELEGRAM_CHAT_ID:-}}"

mkdir -p "$STATE_DIR" "$(dirname "$LOG_FILE")"

VPN_FAIL_FILE="${STATE_DIR}/vpn_fail_streak"
VPN_ALERT_FILE="${STATE_DIR}/vpn_alert_sent"
API_FAIL_FILE="${STATE_DIR}/api_fail_streak"
API_ALERT_FILE="${STATE_DIR}/api_alert_sent"

vpn_streak=0
[ -f "$VPN_FAIL_FILE" ] && vpn_streak=$(cat "$VPN_FAIL_FILE" 2>/dev/null || echo 0)

api_streak=0
[ -f "$API_FAIL_FILE" ] && api_streak=$(cat "$API_FAIL_FILE" 2>/dev/null || echo 0)

send_telegram() {
    local text="$1"
    if [ -z "$TG_AUTH" ] || [ -z "$TG_RECIPIENT" ] || [ "$TG_AUTH" = "REPLACE_WITH_PROBE_BOT_TOKEN" ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] [WARN] Telegram credentials not configured or placeholder. Skipping notification." >> "$LOG_FILE"
        return 0
    fi

    curl -s -m 10 -X POST "https://api.telegram.org/bot${TG_AUTH}/sendMessage" \
        -d "chat_id=${TG_RECIPIENT}" \
        -d "text=${text}" \
        -d "parse_mode=Markdown" > /dev/null 2>&1 || true
}

TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

# ------------------------------------------------------------------------------
# 1. Проверка VPN эндпоинта (HTTPS health или TCP connect на 443)
# ------------------------------------------------------------------------------
vpn_ok=0
vpn_code="000"
vpn_latency_ms=0

# Попытка HTTPS health
http_check=$(curl -s -k -o /dev/null -w "%{http_code}:%{time_total}" -m "$TIMEOUT_SEC" "$VPN_HEALTH_URL" 2>/dev/null || echo "000:0")
h_code=$(echo "$http_check" | cut -d':' -f1)
h_time=$(echo "$http_check" | cut -d':' -f2)

if [ "$h_code" = "200" ]; then
    vpn_ok=1
    vpn_code="200"
    vpn_latency_ms=$(awk "BEGIN {print int($h_time * 1000)}")
else
    # Fallback: прямой TCP connect на рабочий порт 443
    t_start=$(date +%s%3N)
    if nc -z -w "$TIMEOUT_SEC" "$VPN_HOST" "$VPN_PORT" >/dev/null 2>&1; then
        t_end=$(date +%s%3N)
        vpn_ok=1
        vpn_code="TCP_OK"
        vpn_latency_ms=$((t_end - t_start))
    fi
fi

if [ "$vpn_ok" -eq 1 ]; then
    echo "[$TIMESTAMP] [OK] target=VPN endpoint=${VPN_HOST}:${VPN_PORT} code=${vpn_code} latency=${vpn_latency_ms}ms streak=0" >> "$LOG_FILE"
    if [ "$vpn_streak" -ge "$FAIL_THRESHOLD" ] || [ -f "$VPN_ALERT_FILE" ]; then
        MSG="✅ *Sirin VPN: Доступность VPN узла восстановлена*
*Узел:* \`${VPN_HOST}:${VPN_PORT}\`
*Статус:* ${vpn_code}
*Задержка:* ${vpn_latency_ms} ms
*Время:* ${TIMESTAMP}"
        send_telegram "$MSG"
        rm -f "$VPN_ALERT_FILE"
    fi
    echo 0 > "$VPN_FAIL_FILE"
else
    vpn_streak=$((vpn_streak + 1))
    echo "$vpn_streak" > "$VPN_FAIL_FILE"
    echo "[$TIMESTAMP] [FAIL] target=VPN endpoint=${VPN_HOST}:${VPN_PORT} code=${vpn_code} latency=${vpn_latency_ms}ms streak=${vpn_streak}" >> "$LOG_FILE"

    if [ "$vpn_streak" -ge "$FAIL_THRESHOLD" ] && [ ! -f "$VPN_ALERT_FILE" ]; then
        MSG="🚨 *ВНИМАНИЕ: Сбой доступности Sirin VPN из РФ!*
*Узел:* \`${VPN_HOST}:${VPN_PORT}\`
*Ошибка:* Недоступен по TCP/HTTPS (серия сбоев: ${vpn_streak})
*Время:* ${TIMESTAMP}
_Возможна блокировка IP-адреса шлюза или сбой демона на узле._"
        send_telegram "$MSG"
        touch "$VPN_ALERT_FILE"
    fi
fi

# ------------------------------------------------------------------------------
# 2. Проверка API эндпоинта (HTTP GET /health)
# ------------------------------------------------------------------------------
api_ok=0
api_check=$(curl -s -k -o /dev/null -w "%{http_code}:%{time_total}" -m "$TIMEOUT_SEC" "$API_URL" 2>/dev/null || echo "000:0")
api_code=$(echo "$api_check" | cut -d':' -f1)
api_time=$(echo "$api_check" | cut -d':' -f2)
api_latency_ms=$(awk "BEGIN {print int($api_time * 1000)}")

if [ "$api_code" = "200" ]; then
    api_ok=1
    echo "[$TIMESTAMP] [OK] target=API endpoint=${API_URL} code=${api_code} latency=${api_latency_ms}ms streak=0" >> "$LOG_FILE"
    if [ "$api_streak" -ge "$FAIL_THRESHOLD" ] || [ -f "$API_ALERT_FILE" ]; then
        MSG="✅ *Sirin VPN: Доступность API восстановлена*
*Эндпоинт:* \`${API_URL}\`
*Статус:* HTTP 200 OK
*Задержка:* ${api_latency_ms} ms
*Время:* ${TIMESTAMP}"
        send_telegram "$MSG"
        rm -f "$API_ALERT_FILE"
    fi
    echo 0 > "$API_FAIL_FILE"
else
    api_streak=$((api_streak + 1))
    echo "$api_streak" > "$API_FAIL_FILE"
    echo "[$TIMESTAMP] [FAIL] target=API endpoint=${API_URL} code=${api_code} latency=${api_latency_ms}ms streak=${api_streak}" >> "$LOG_FILE"

    if [ "$api_streak" -ge "$FAIL_THRESHOLD" ] && [ ! -f "$API_ALERT_FILE" ]; then
        MSG="🚨 *ВНИМАНИЕ: Сбой доступности Sirin API из РФ!*
*Эндпоинт:* \`${API_URL}\`
*Ошибка:* HTTP ${api_code} (серия сбоев: ${api_streak})
*Время:* ${TIMESTAMP}
_Возможен сбой домена, TLS-сертификата или блокировка API._"
        send_telegram "$MSG"
        touch "$API_ALERT_FILE"
    fi
fi

if [ "$vpn_ok" -eq 1 ] && [ "$api_ok" -eq 1 ]; then
    exit 0
else
    exit 1
fi
