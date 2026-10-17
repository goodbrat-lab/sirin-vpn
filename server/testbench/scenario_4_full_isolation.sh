#!/usr/bin/env bash
# Сценарий 4: полная изоляция (только ICMP + DNS)
set -euo pipefail
sudo ip netns exec testbench nft add table ip filter
sudo ip netns exec testbench nft add chain ip filter output '{ type filter hook output priority 0; policy drop; }'
sudo ip netns exec testbench nft add rule ip filter output oif lo accept
sudo ip netns exec testbench nft add rule ip filter output ip protocol icmp accept
sudo ip netns exec testbench nft add rule ip filter output ip daddr 8.8.8.8 udp dport 53 accept
echo "Testbench scenario 4: full isolation (ICMP + DNS only)"
