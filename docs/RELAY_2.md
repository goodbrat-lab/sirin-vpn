# Европейский узел Relay 2 (Германия, Франкфурт)

Документ описывает назначение, архитектуру, конфигурацию потокового ретранслятора (TLS passthrough) и регламент эксплуатации второго релея Sirin VPN в рамках многопутевой отказоустойчивой архитектуры.

---

## 1. Спецификация узла

| Параметр | Значение |
|---|---|
| **IP-адрес** | `45.38.228.142` |
| **Домен** | `media.guseynovia.ru` |
| **Локация** | Германия, Франкфурт-на-Майне |
| **RTT до NL (94.183.159.114)** | **15.9–16.4 мс** (0% потерь) |
| **RTT до Москвы (185.182.65.219)** | **51.5–55.6 мс** (0% потерь) |
| **Операционная система** | Ubuntu 24.04 LTS (Noble Numbat, x86_64) |
| **Сосуществующие сервисы** | 1. Xray Reality (`443/tcp`) — персональный sing-box для Instagram/AI.<br>2. AdGuard Home DoH (`8443/tcp` via Nginx) — DNS over HTTPS.<br>3. 3X-UI Web (`2053/tcp`, `2096/tcp`) — ограничен Tailscale.<br>4. Tailscale VPN (`100.116.244.79`). |
| **Роли Sirin** | 1. **Relay 2 (Германия)**: Nginx stream passthrough на порты `9443` (VLESS) и `9444` (XHTTP).<br>2. **Watchdog Node**: мониторинг RTT/loss до NL и Москвы раз в 60 с. |
| **Сетевые порты Relay 2** | `9443/tcp` (VLESS Reality passthrough), `9444/tcp` (XHTTP passthrough) |

---

## 2. Архитектура и изоляция сервисов

Сервер `45.38.228.142` выполняет двойную функцию: несёт персональные сервисы пользователя и служит резервным высокоскоростным релеем для Sirin VPN.

Поскольку порты `443` (Xray) и `8443` (Nginx AdGuard DoH) уже заняты, для Relay 2 выделены независимые порты:
- **`9443/tcp`** → проксирует TCP-поток на `94.183.159.114:443` (VLESS Reality).
- **`9444/tcp`** → проксирует TCP-поток на `94.183.159.114:8443` (XHTTP).

### Схема прохождения трафика

```
Клиент Sirin (при relay_id=2 или при degraded fallback)
       │
       ▼ (TLS 1.3 Reality, SNI=sun8-21.userapi.com)
45.38.228.142:9443 (Relay 2, Германия)
       │ (Чистый TCP stream passthrough без расшифровки)
       ▼ (16 мс RTT, 0% потерь)
94.183.159.114:443 (NL VPN Egress)
       │
       ▼
Интернет (Instagram / YouTube / AI)
```

---

## 3. Конфигурация Nginx Stream

Файл: `/etc/nginx/streams-enabled/relay2.conf`

```nginx
# Sirin VPN - Relay 2 (Germany) TLS Passthrough Stream Forwarding
# Pure TCP proxy to NL egress (94.183.159.114)
# Ports 9443 (VLESS) and 9444 (XHTTP)

upstream sirin_nl_vless {
    server 94.183.159.114:443;
}

server {
    listen 9443;
    listen [::]:9443;
    proxy_pass sirin_nl_vless;
    tcp_nodelay on;
    proxy_socket_keepalive on;
    proxy_connect_timeout 5s;
    proxy_timeout 1h;
}

upstream sirin_nl_xhttp {
    server 94.183.159.114:8443;
}

server {
    listen 9444;
    listen [::]:9444;
    proxy_pass sirin_nl_xhttp;
    tcp_nodelay on;
    proxy_socket_keepalive on;
    proxy_connect_timeout 5s;
    proxy_timeout 1h;
}
```

Включение в `/etc/nginx/nginx.conf`:
```nginx
stream {
    include /etc/nginx/streams-enabled/*.conf;
}
```

---

## 4. Конфигурация брандмауэра (UFW) и OpSec

На сервере применены строгие правила изоляции:
1. SSH (`22/tcp`): только по ключам (`ed25519`), `PasswordAuthentication no`, `PermitRootLogin prohibit-password`.
2. 3X-UI (`2053/tcp`, `2096/tcp`): закрыты из публичного интернета (`DENY IN`), доступны только через интерфейс Tailscale (`tailscale0`).
3. Fail2ban: активен для `sshd` и `3x-ipl`, кластерные IP (`94.183.159.114`, `185.182.65.219`, `45.38.228.142`) и Tailscale подсеть внесены в `ignoreip`.
4. Relay 2: порты `9443/tcp` и `9444/tcp` открыты для внешних подключений.

---

## 5. Регистрация в `relays.json` на NL

Файл `/etc/sirin/relays.json`:
```json
{
  "relays": [
    {
      "id": 1,
      "name": "Relay 1 (Москва, RU)",
      "host": "185.182.65.219",
      "port_vless": 443,
      "port_xhttp": 8443,
      "max_clients": 10,
      "enabled": true
    },
    {
      "id": 2,
      "name": "Германия (45.38.228.142)",
      "host": "45.38.228.142",
      "port_vless": 9443,
      "port_xhttp": 9444,
      "max_clients": 10,
      "enabled": true
    }
  ]
}
```
