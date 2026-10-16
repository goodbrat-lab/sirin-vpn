# Shared Resources

Общие ресурсы для всех клиентов Sirin (Android, Desktop, Router).

## Структура

- geoip/ — списки российских IP-диапазонов (RIPE NCC)
  - ru-ranges.txt — 8641 CIDR-блок, обновляется из RIPE
- geosite/ — списки российских доменов
  - ru-domains.txt — домены категории "ru" (v2fly)
  - bypass-domains.txt — исключения для прямого доступа
- assets/ — UI-ресурсы
  - desktop/ — для desktop-клиента (Wails v3)
  - icons/ — общие иконки

## Источник

Все файлы адаптированы для проекта Sirin.

## Обновление

- ru-ranges.txt — генерируется из RIPE NCC (скрипт в Фазе 5)
- ru-domains.txt — из v2fly/domain-list-community
- bypass-domains.txt — вручную
