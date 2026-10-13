#!/usr/bin/env bash
set -euo pipefail

# 1. Включить IP forwarding ядра Linux
sudo sysctl -w net.ipv4.ip_forward=1
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf

# 2. NAT/Masquerade для трафика клиентов из подсети 10.77.77.0/24
sudo nft add table ip sirin-nat
sudo nft add chain ip sirin-nat postrouting '{ type nat hook postrouting priority srcnat; policy accept; }'
sudo nft add rule ip sirin-nat postrouting ip saddr 10.77.77.0/24 oifname != "sirin0" masquerade
