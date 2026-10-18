# Сирин (Sirin)

VPN с защитой от блокировок ТСПУ. Собственный сервер + Android-клиент.

## Что это

- Два транспорта с автоматическим fallback:
  - UDP Noise NK (быстрый) на порту 56005
  - VLESS+Reality (XTLS Vision) на порту 443
- Маскировка под VK CDN (sun8-21.userapi.com)
- Автоматический fallback при блокировке UDP (392 мс)
- Split tunneling: российские приложения — напрямую, остальное — через VPN
- Android-клиент + API + Telegram-бот

## Архитектура

```text
┌─────────┐       ┌──────────────────┐       ┌──────────┐
│ Android │──────>│        VPS       │──────>│ Интернет │
│  Сирин  │       │ ┌──────────────┐ │       └──────────┘
└─────────┘       │ │ sirind : port│ │
                  │ │ sirinapi:port│ │
                  │ │ sirinbot     │ │
                  │ │ xray   :port │ │
                  │ │ nginx  :port │ │
                  │ └──────────────┘ │
                  └──────────────────┘
```

## Компоненты

| Компонент | Язык | Назначение |
|---|---|---|
| core/engine/ | Go | Ядро: протокол, ladder, Noise NK, VLESS |
| apps/android/ | Kotlin | Android-клиент |
| apps/desktop/ | Go + Wails | Desktop-клиент (Windows) |
| apps/router/ | Go | OpenWrt-клиент (5 архитектур) |
| server/cmd/sirind/ | Go | UDP/TCP VPN-сервер |
| server/cmd/sirinapi/ | Go | HTTP API (/v1/key, /v1/params) |
| server/cmd/sirinbot/ | Go | Telegram-бот (@sirin_crp_bot) |
| shared/ | — | Geo-базы, UI-ресурсы, брендинг |

## Быстрый старт (Android)

1. Установи APK (`apps/android/app/build/outputs/apk/direct/debug/app-direct-debug.apk`)
2. Открой приложение «Сирин»
3. Введи UUID (получи у администратора)
4. Нажми «Применить» → «Подключить»

## Сервер (Ubuntu 24.04)

Развёртывание — см. документацию в `server/` (в разработке: Docker).

## Router (OpenWrt)

Клиент для роутеров на OpenWrt 24.x / 25.x.

### Установка

```bash
wget -O - https://raw.githubusercontent.com/goodbrat-lab/sirin-vpn/master/apps/router/install.sh | sh
```

Или вручную: скачать `sirin-client-<version>-<arch>.tar.gz` из Releases, распаковать, запустить `install.sh`.

### Архитектуры

| Архитектура | Где используется |
|---|---|
| mipsle | Xiaomi 4A, TP-Link, Keenetic (большинство) |
| armv7 | Keenetic Hopper, Xiaomi AX1800 |
| arm64 | Raspberry Pi, NanoPi, Keenetic Titan |
| x86_64 | Мини-ПК |

### Настройка

1. Отредактировать `/etc/sirin/client.toml` — вставить UUID из @sirin_crp_bot
2. Запустить: `/etc/init.d/sirin start`
3. Проверить: `logread -f | grep sirin`

## Credits

Этот проект основан на Meridian VPN (JinComputers),
распространявшемся под MIT License.
См. NOTICE и LICENSE для деталей.

## Лицензия

MIT — см. LICENSE.
