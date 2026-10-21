# Sirin iOS Spike (Pre-Flight Prototype)

Прототип для проверки главного риска iOS-клиента Sirin VPN: **лимит памяти 50 MiB в `NEPacketTunnelProvider`** и валидация связки `gomobile + Reality (uTLS) + UDP Noise NK`.

> **ВАЖНО:** Данный каталог содержит исследовательский прототип (spike), а НЕ production-код MVP. Кодовые базы Android, Desktop, Router и `core/engine/` не модифицируются.

---

## Структура каталога

```
apps/ios-spike/
├── mobile/                      # Go-обёртка над core/engine/desktop
│   ├── go.mod                   # Зависимость от sirin/engine через replace
│   └── mobile.go                # Экспорт SirinStart/Stop, PacketFlow и лимит памяти 38 MiB
├── SirinSpike/                  # Минимальное SwiftUI приложение
│   ├── SirinSpikeApp.swift      # Точка входа, обработка sirin:// deep-link
│   ├── ContentView.swift        # UI: кнопка Connect + ввод UUID + статус + консоль логов
│   ├── VPNManager.swift         # Управление NETunnelProviderManager, App Group
│   ├── Info.plist               # URL scheme sirin://
│   └── SirinSpike.entitlements  # App Group group.ru.guseynovia.sirin.spike
├── SirinSpikeTunnel/            # Сетевое расширение PacketTunnelProvider
│   ├── PacketTunnelProvider.swift # Мост NEPacketTunnelFlow ↔ Go PacketFlow
│   ├── Info.plist               # Packet-tunnel extension point
│   └── SirinSpikeTunnel.entitlements
├── build-xcframework.sh         # Скрипт сборки Mobile.xcframework через gomobile
└── README.md                    # Инструкции и методика замеров
```

---

## Пошаговая инструкция запуска и замера на macOS

### 1. Сборка фреймворка
```bash
cd apps/ios-spike
chmod +x build-xcframework.sh
./build-xcframework.sh
```
В результате создается `apps/ios-spike/Mobile.xcframework`.

### 2. Замер памяти (Memory Profiling) в Xcode Instruments
1. Открыть проект в Xcode на Mac.
2. Выбрать схему `SirinSpikeTunnel` -> целевое устройство (физический iPhone или симулятор).
3. Перейти в **Product -> Profile -> Instruments -> Allocations & VM Tracker**.
4. Ввести UUID тестового клиента в приложении и нажать **Connect**.
5. Зафиксировать:
   - **Baseline RSS (простой):** норма 18–25 MiB.
   - **Peak RSS (под нагрузкой):** запустить Speedtest / 4K-стрим на 5–10 минут.
   - **Jetsam watchdog threshold:** если пик превышает **42 MiB**, процесс в зоне риска `ActiveHard 50 MB`.
6. Мониторинг логов в терминале:
   ```bash
   log stream --predicate 'subsystem == "ru.guseynovia.sirin.spike"'
   ```

### 3. Проверка Reality + uTLS
1. В логах найти строку:  
   `connection established (transport: vless)`
2. Проверить внешний IP через браузер: `https://ip.me` (должен показывать IP шлюза `94.183.159.114`).
3. Убедиться, что uTLS эмулирует Chrome 120+ без сбросов соединения ТСПУ.
