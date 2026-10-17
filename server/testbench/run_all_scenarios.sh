#!/usr/bin/env bash
# run_all_scenarios.sh — запускает все 4 сценария + baseline
set -euo pipefail

cd /opt/testbench
mkdir -p /opt/testbench/logs

echo "========================================="
echo "=== Test 0: Baseline (no blocking) ==="
echo "========================================="
./scenario_reset.sh
./test_vless_direct.sh

echo ""
echo "========================================="
echo "=== Test 1: Block UDP 56005 ==="
echo "========================================="
./scenario_reset.sh
./scenario_1_block_udp.sh
./test_vless_direct.sh  # должен работать (fallback)
echo "UDP check:"
sudo ip netns exec testbench timeout 3 nc -zu 94.183.159.114 56005 2>&1 | head -2 || true

echo ""
echo "========================================="
echo "=== Test 2: Block TCP 443 ==="
echo "========================================="
./scenario_reset.sh
./scenario_2_block_sni.sh
echo "TCP 443 check (must fail):"
sudo ip netns exec testbench curl -k --max-time 3 https://94.183.159.114/ 2>&1 | head -3 || true

echo ""
echo "========================================="
echo "=== Test 3: Whitelist mode ==="
echo "========================================="
./scenario_reset.sh
./scenario_3_whitelist.sh
echo "Direct google.com (must fail):"
sudo ip netns exec testbench curl -sf --max-time 3 https://google.com/ && echo "FAIL: google accessible" || echo "OK: google blocked"
echo "VK (must work):"
sudo ip netns exec testbench curl -sf --max-time 3 https://vk.com/ > /dev/null && echo "OK: vk accessible" || echo "WARN: vk blocked too"

echo ""
echo "========================================="
echo "=== Test 4: Full isolation ==="
echo "========================================="
./scenario_reset.sh
./scenario_4_full_isolation.sh
echo "Ping 8.8.8.8 (must work):"
sudo ip netns exec testbench ping -c 1 -W 2 8.8.8.8 > /dev/null && echo "OK" || echo "FAIL"
echo "curl google (must fail):"
sudo ip netns exec testbench curl -sf --max-time 3 https://google.com/ && echo "FAIL" || echo "OK: blocked"

# Сброс
./scenario_reset.sh
echo ""
echo "=== All scenarios complete ==="
