#!/usr/bin/env bash
# Проверка, что VPN доступен БЕЗ блокировок (baseline)
set -euo pipefail

echo "=== Baseline test: UDP Noise NK direct ==="
sudo ip netns exec testbench curl -sf --max-time 10 https://crp.guseynovia.ru/health && echo " API OK" || echo " API FAIL"
echo ""
echo "Test from testbench to VPN server (UDP 56005):"
sudo ip netns exec testbench timeout 5 nc -u -v 94.183.159.114 56005 < /dev/null 2>&1 | head -3 || true
