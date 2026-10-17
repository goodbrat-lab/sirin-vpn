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
│ Android │──────>│ VPS 94.183.159.114│──────>│ Интернет │
│  Сирин  │       │ ┌──────────────┐ │       └──────────┘
└─────────┘       │ │ sirind :56005│ │
                  │ │ sirinapi:8080│ │
                  │ │ sirinbot     │ │
                  │ │ xray   :4430 │ │
                  │ │ nginx  :443  │ │
                  │ └──────────────┘ │
                  └──────────────────┘
```

## Компоненты

| Компонент | Язык | Назначение |
|---|---|---|
| core/engine/ | Go | Ядро: протокол, ladder, Noise NK, VLESS |
| apps/android/ | Kotlin | Android-клиент |
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

## Credits

Этот проект основан на Meridian VPN (JinComputers),
распространявшемся под MIT License.
См. NOTICE и LICENSE для деталей.

## Лицензия

MIT — см. LICENSE.
