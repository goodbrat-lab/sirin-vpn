#!/usr/bin/env bash
# Сценарий 2: блокировка TCP 443 (VLESS Reality)
# Клиент не должен иметь возможности подключиться, если UDP тоже заблокирован
set -euo pipefail
sudo ip netns exec testbench nft add table ip filter
sudo ip netns exec testbench nft add chain ip filter output '{ type filter hook output priority 0; policy accept; }'
sudo ip netns exec testbench nft add rule ip filter output tcp dport 443 drop
echo "Testbench scenario 2: TCP 443 blocked (VLESS Reality)"
