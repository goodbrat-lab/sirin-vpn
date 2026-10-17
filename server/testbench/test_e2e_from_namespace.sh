#!/usr/bin/env bash
# Запуск e2e-test (CLI) ИЗ namespace testbench
# Проверяет полный протокол (Reality -> VLESS -> streamlink -> Noise -> AUTH)
set -euo pipefail

if [ ! -f /tmp/e2e-test ]; then
  echo "ERROR: /tmp/e2e-test не найден. Собрать: cd core/engine && go build -o /tmp/e2e-test ./vless/cmd/e2e-test"
  exit 1
fi

mkdir -p /opt/testbench/logs
sudo ip netns exec testbench /tmp/e2e-test --via-ladder 2>&1 | tee /opt/testbench/logs/e2e_$(date +%s).log
