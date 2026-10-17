#!/usr/bin/env bash
# Проверка VLESS Reality (TCP 443) из testbench
set -euo pipefail

echo "=== VLESS Reality test (TCP 443) ==="
# Используем curl для проверки, что TLS+Reality работает
sudo ip netns exec testbench curl -sf -k --max-time 10 \
  --resolve sun8-21.userapi.com:443:94.183.159.114 \
  https://sun8-21.userapi.com/ 2>&1 | head -5 || echo "FAIL"
