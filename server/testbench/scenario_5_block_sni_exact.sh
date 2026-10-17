#!/usr/bin/env bash
# Сценарий 5: точная блокировка SNI через NFQUEUE
set -euo pipefail
sudo ip netns exec testbench nft flush ruleset
# Перенаправляем весь исходящий TCP трафик на порт 443 в очередь №1
sudo ip netns exec testbench nft add table ip filter
sudo ip netns exec testbench nft add chain ip filter output '{ type filter hook output priority 0; policy accept; }'
sudo ip netns exec testbench nft add rule ip filter output tcp dport 443 queue num 1
echo "Testbench scenario 5: SNI-based blocking (NFQUEUE) active"
