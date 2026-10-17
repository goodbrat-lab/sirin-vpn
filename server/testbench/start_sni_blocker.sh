#!/usr/bin/env bash
set -euo pipefail
pkill -f sni_blocker.py || true
ip netns exec testbench nohup python3 -u /opt/testbench/sni_blocker.py > /opt/testbench/sni_blocker.log 2>&1 &
sleep 1
cat /opt/testbench/sni_blocker.log
