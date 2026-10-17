#!/usr/bin/env bash
# Сброс всех правил в namespace testbench
set -euo pipefail
sudo ip netns exec testbench nft flush ruleset
echo "Testbench: rules reset"
