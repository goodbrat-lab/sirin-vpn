#!/usr/bin/env bash
# Сценарий 1: блокировка UDP к нашему серверу (56005)
# Клиент должен переключиться на VLESS TCP 443 (fallback)
set -euo pipefail
sudo ip netns exec testbench nft add table ip filter
sudo ip netns exec testbench nft add chain ip filter output '{ type filter hook output priority 0; policy accept; }'
# Дроп только UDP на 56005 (наш Noise NK)
sudo ip netns exec testbench nft add rule ip filter output udp dport 56005 drop
echo "Testbench scenario 1: UDP 56005 blocked"
