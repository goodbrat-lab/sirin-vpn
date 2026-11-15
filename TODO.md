# TODO

## Windows Desktop Client
- [x] RuDirect (8641 CIDR RIPE NCC через Win32 IP Helper API)
- [x] Раздельное туннелирование по доменам и IP-диапазонам (Bypass / Only)
- [x] UI настроек маршрутизации и списков исключений в Wails v3
- [x] In-app Updater / автообновление прямо из интерфейса (`updater.go`)
- [x] Встроенный движок Zapret 2 (`tunZapretInterceptor`) + Lazy On-Demand VPS VPN (`v1.7.0`)
- [ ] Регистрация системного URI-обработчика `sirin://` в реестре Windows

## Router & LuCI Web UI
- [x] Автономная сборка пакетов `.ipk` (opkg) и `.apk` (OpenWrt 25.x apk) для всех 5 архитектур + универсальный `luci-app-sirin`
- [x] OpenWrt SDK Makefiles (`openwrt/sirin-client/Makefile` и `openwrt/luci-app-sirin/Makefile`)
- [x] All-in-one бандлы `sirin-full` для установки одной командой
- [x] Split tunneling по номерам портов (порты, диапазоны, префиксы tcp/udp, режимы Only/Bypass для игр)
- [x] OpSec & Network Stack Hardening (нормализация TTL 64/128, TCP MSS clamping, Anti-DNS-Leak dstnat, блокировка DoT 853)
- [x] Проверка и верификация на реальном роутере OpenWrt 24.10 (`aarch64` / `opkg`)
- [x] Встроенное обновление пакета в один клик прямо из LuCI (`check_update` / `upgrade`)
- [ ] Мультиязычная локализация LuCI UI (`po/ru`, `_()` в `overview.js`)

## Advanced Anti-Censorship & OpSec Roadmap
- [x] **Этап 1: OpSec & Network Stack Hardening** (нормализация TTL, перехват DNS, защита от DoT, uTLS Reality)
- [x] **Этап 2: Dead Drop конфигураций и параметров** (Яндекс Диск / S3-совместимое хранилище как альтернатива прямому обращению к серверу)
- [x] **Этап 3: Anti-DPI для YouTube, Instagram, Discord и X** (многоступенчатый движок Zapret 2 без прогона трафика через VPN)
- [x] **Этап 4: Аварийный Whitelist Fallback** (резервный канал через VK Calls / DION WebRTC TURN при жестких блокировках и белых списках)
- [x] **Этап 5: Гибридный Zapret 2 + Lazy On-Demand VPS VPN (`v1.7.0`)** (спящий режим VPS с 0 Байт холостого трафика и автоматическим пробуждением туннеля при отказе Anti-DPI или обращении к гео-заблокированным ИИ-сервисам)
