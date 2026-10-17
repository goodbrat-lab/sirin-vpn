#!/usr/bin/env bash
# Сценарий 3: белые списки (только VK, Яндекс, Госуслуги)
# Всё остальное — drop
set -euo pipefail
sudo ip netns exec testbench nft add table ip filter
sudo ip netns exec testbench nft add chain ip filter output '{ type filter hook output priority 0; policy drop; }'
# Разрешаем loopback
sudo ip netns exec testbench nft add rule ip filter output oif lo accept
# Разрешаем established/related
sudo ip netns exec testbench nft add rule ip filter output ct state established,related accept
# Разрешаем DNS (8.8.8.8, 1.1.1.1)
sudo ip netns exec testbench nft add rule ip filter output ip daddr 8.8.8.8 udp dport 53 accept
sudo ip netns exec testbench nft add rule ip filter output ip daddr 1.1.1.1 udp dport 53 accept
# Разрешаем VK (основные подсети AS47541)
sudo ip netns exec testbench nft add rule ip filter output ip daddr 87.240.0.0/16 accept
sudo ip netns exec testbench nft add rule ip filter output ip daddr 93.186.225.0/24 accept
sudo ip netns exec testbench nft add rule ip filter output ip daddr 217.20.147.0/24 accept
sudo ip netns exec testbench nft add rule ip filter output ip daddr 87.250.250.0/24 accept
sudo ip netns exec testbench nft add rule ip filter output ip daddr 213.180.204.0/24 accept
sudo ip netns exec testbench nft add rule ip filter output ip daddr 109.207.11.0/24 accept
echo "Testbench scenario 3: whitelist mode (VK/Yandex/Gosuslugi only)"
