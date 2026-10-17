#!/usr/bin/env bash
# setup.sh — создаёт namespace testbench
set -euo pipefail

if [ "$EUID" -ne 0 ]; then
    echo "ERROR: запускать от root"
    exit 1
fi

# Создать namespace
ip netns add testbench 2>/dev/null || true

# veth-пара
ip link add veth-host type veth peer name veth-tb 2>/dev/null || true
ip link set veth-tb netns testbench

# IP-адреса
ip addr add 192.168.99.1/24 dev veth-host
ip link set veth-host up
ip netns exec testbench ip addr add 192.168.99.2/24 dev veth-tb
ip netns exec testbench ip link set veth-tb up
ip netns exec testbench ip link set lo up

# Маршрут по умолчанию
ip netns exec testbench ip route add default via 192.168.99.1

# DNS
mkdir -p /etc/netns/testbench
echo "nameserver 8.8.8.8" > /etc/netns/testbench/resolv.conf
echo "nameserver 1.1.1.1" >> /etc/netns/testbench/resolv.conf

# NAT
nft add table ip testbench-nat
nft add chain ip testbench-nat postrouting '{ type nat hook postrouting priority srcnat; }'
nft add rule ip testbench-nat postrouting ip saddr 192.168.99.0/24 oifname eth0 masquerade
nft add rule inet filter forward iifname "veth-host" oifname "eth0" accept
nft add rule inet filter forward iifname "eth0" oifname "veth-host" ct state established,related accept

# Проверка
echo "Setup complete. Testing:"
ip netns exec testbench ping -c 2 8.8.8.8
