# Testbench — эмуляция ТСПУ

Изолированный network namespace для проверки устойчивости Sirin VPN к блокировкам РКН.

## Требования

- Linux (Ubuntu 24.04)
- root (для `ip netns`)
- nftables, python3

## Установка

```bash
sudo ./setup.sh
```

Создаёт `ip netns testbench` с veth-парой (`192.168.99.1` ↔ `192.168.99.2`) и NAT в интернет.

## Сценарии

| Скрипт | Что блокирует |
|---|---|
| `scenario_reset.sh` | Сбрасывает все правила |
| `scenario_1_block_udp.sh` | UDP 56005 (Noise NK) |
| `scenario_2_block_sni.sh` | TCP 443 (VLESS Reality) |
| `scenario_3_whitelist.sh` | Всё, кроме VK/Яндекс/Госуслуги |
| `scenario_4_full_isolation.sh` | Всё, кроме ICMP + DNS |

## Запуск

```bash
sudo ./run_all_scenarios.sh
```

## Результаты (2026-10-17)

- ✅ Baseline: VLESS работает
- ✅ Block UDP: fallback на VLESS (475ms)
- ✅ Block TCP 443: дроп подтверждён
- ✅ Whitelist: Google blocked, VK accessible
- ✅ Full isolation: ping OK, HTTPS blocked
- ✅ E2E из namespace: FULL LADDER PASSED

## Важно

Все блокировки только внутри namespace, production не трогается.

- Сброс: `sudo ip netns delete testbench`
- Полный сброс правил: `sudo ./scenario_reset.sh`

## Ограничения

Сценарий 2 (`block_sni.sh`) блокирует весь TCP 443, а не конкретный SNI. Для точной эмуляции SNI нужен NFQUEUE + парсер TLS ClientHello. Текущая версия проверяет: «если VLESS-транспорт полностью недоступен».
