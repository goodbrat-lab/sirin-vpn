# TODO

## Windows Desktop Client
- [x] RuDirect (8641 CIDR RIPE NCC через Win32 IP Helper API)
- [x] Раздельное туннелирование по доменам и IP-диапазонам (Bypass / Only)
- [x] UI настроек маршрутизации и списков исключений в Wails v3
- [ ] In-app Updater / автообновление
- [ ] Оформление Cyber-Neon в стиле Android-клиента

## Router & LuCI Web UI
- [x] Автономная сборка пакетов `.ipk` (opkg) и `.apk` (OpenWrt 25.x apk) для всех 5 архитектур + универсальный `luci-app-sirin`
- [x] OpenWrt SDK Makefiles (`openwrt/sirin-client/Makefile` и `openwrt/luci-app-sirin/Makefile`)
- [x] All-in-one бандлы `sirin-full` для установки одной командой
- [x] Split tunneling по номерам портов (порты, диапазоны, префиксы tcp/udp, режимы Only/Bypass для игр)
- [x] OpSec & Network Stack Hardening (нормализация TTL 64/128, TCP MSS clamping, Anti-DNS-Leak dstnat, блокировка DoT 853)
- [ ] i18n UI (po/ru, `_()` в `overview.js`)
- [ ] Проверка на реальном OpenWrt 24.x (opkg) и 25.x (apk)

## Advanced Anti-Censorship & OpSec Roadmap
- [x] **Этап 1: OpSec & Network Stack Hardening** (нормализация TTL, перехват DNS, защита от DoT, uTLS Reality)
- [x] **Этап 2: Dead Drop конфигураций и параметров** (Яндекс Диск / S3-совместимое хранилище как альтернатива прямому обращению к серверу)
- [x] **Этап 3: Anti-DPI для YouTube и Discord** (стратегии фрагментации ClientHello, TCP Window manipulation без прогона через VPN)
- [x] **Этап 4: Аварийный Whitelist Fallback** (резервный канал через VK Cloud S3 / DION WebRTC TURN при жестких блокировках)
