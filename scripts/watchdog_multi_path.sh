#!/bin/bash
# Sirin VPN - Multi-Path Latency & Packet Loss Watchdog
# Monitors network routes between NL egress, Moscow relay, and Germany relay.
# Reports telemetry to Sirin API (/v1/admin/monitor).
#
# Configuration is read from /etc/sirin/monitor.env (mode 0600).

ROLE="${1:-auto}"
API_URL="${API_URL:-https://crp.guseynovia.ru}"
ENV_FILE="/etc/sirin/monitor.env"

if [ -f "$ENV_FILE" ]; then
    # shellcheck disable=SC1090
    source "$ENV_FILE"
fi

if [ -z "$ADMIN_TOKEN" ]; then
    echo "Error: ADMIN_TOKEN must be defined in $ENV_FILE or environment"
    exit 1
fi

# Cluster host targets from env (/etc/sirin/monitor.env)
NL_HOST="${NL_IP:-}"
MOSCOW_HOST="${MOSCOW_IP:-}"
GERMANY_HOST="${GERMANY_IP:-}"

# Auto-detect role by checking local IP addresses
if [ "$ROLE" = "auto" ]; then
    MY_IPS=$(ip -br a 2>/dev/null | awk '{print $3}' | cut -d'/' -f1)
    if echo "$MY_IPS" | grep -qw "$GERMANY_HOST"; then
        ROLE="germany"
    elif echo "$MY_IPS" | grep -qw "$MOSCOW_HOST"; then
        ROLE="moscow"
    elif echo "$MY_IPS" | grep -qw "$NL_HOST"; then
        ROLE="nl"
    else
        ROLE="germany"
    fi
fi

probe_node() {
    local target_ip="$1"
    local node_label="$2"
    local health_url="$3"

    [ -z "$target_ip" ] && return 0

    # Ping 10 packets with 1s timeout
    local ping_out
    ping_out=$(ping -c 10 -W 2 -q "$target_ip" 2>&1)
    local ping_code=$?

    local loss=100
    local rtt=999.0

    if [ $ping_code -eq 0 ]; then
        loss=$(echo "$ping_out" | grep -oE '[0-9]+% packet loss' | tr -dc '0-9')
        [ -z "$loss" ] && loss=0

        local rtt_avg
        rtt_avg=$(echo "$ping_out" | grep -oE 'rtt min/avg/max/mdev = [0-9.]+/([0-9.]+)' | cut -d'/' -f5)
        if [ -n "$rtt_avg" ]; then
            rtt="$rtt_avg"
        fi
    fi

    local status="ok"
    if [ "$loss" -gt 5 ]; then
        status="degraded"
    fi
    local rtt_check
    rtt_check=$(awk -v r="$rtt" 'BEGIN { print (r > 150.0) ? 1 : 0 }')
    if [ "$rtt_check" -eq 1 ]; then
        status="degraded"
    fi

    if [ -n "$health_url" ]; then
        local http_code
        http_code=$(curl -s -k -m 5 -o /dev/null -w "%{http_code}" "$health_url" 2>/dev/null || echo "000")
        if [ "$http_code" != "200" ] && [ "$status" = "ok" ]; then
            status="degraded"
        fi
    fi

    echo "$(date -u +'%Y-%m-%dT%H:%M:%SZ') [Watchdog] probe $node_label ($target_ip): RTT=${rtt}ms, Loss=${loss}%, Status=${status}"

    local payload
    payload=$(printf '{"node":"%s","rtt_ms":%s,"loss_pct":%s,"status":"%s"}' "$node_label" "$rtt" "$loss" "$status")

    curl -s -X POST \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$API_URL/v1/admin/monitor" > /dev/null || true
}

case "$ROLE" in
    germany)
        probe_node "$NL_HOST" "germany" "https://crp.guseynovia.ru/health"
        probe_node "$MOSCOW_HOST" "moscow_from_germany" ""
        ;;
    moscow)
        probe_node "$NL_HOST" "moscow" "https://crp.guseynovia.ru/health"
        probe_node "$GERMANY_HOST" "germany_from_moscow" ""
        ;;
    nl)
        probe_node "$MOSCOW_HOST" "nl_to_moscow" ""
        probe_node "$GERMANY_HOST" "nl_to_germany" ""
        ;;
    *)
        echo "Unknown role: $ROLE"
        exit 1
        ;;
esac
