# Дорожная карта проекта Sirin VPN (Roadmap)

В данном документе зафиксированы ключевые направления развития клиентских приложений, протокола ядра и серверной инфраструктуры Sirin VPN.

---

## 🎯 Текущий статус по компонентам

### 1. Router Client (OpenWrt)
- [x] **v1.0.0**: Базовый full tunnel и встроенный RuDirect (4945 CIDR).
- [x] **v1.1.0–v1.1.2**: Веб-интерфейс LuCI, поддержка PPPoE WAN, автоопределение шлюза, автозапуск procd.
- [x] **v1.2.0**: Настраиваемое раздельное туннелирование (режимы Bypass и Only, произвольные CIDR, доменные имена, 8641 CIDR в RuDirect, атомарная загрузка в nftables sets).
- [x] **v1.3.0–v1.4.0**:
  - XHTTP+Reality транспорт (Router v1.3.0) и встроенный Kill Switch (v1.3.2).
  - Раздельное туннелирование по портам (Router v1.4.0): направление выбранных портов через VPN, поддержка диапазонов и протоколов (`tcp:80`, `udp:27000-27050`), режим Gaming Bypass.
- [x] **v1.5.0**:
  - Автономная сборка пакетов `.ipk` (OpenWrt <= 24.x, `opkg`) и `.apk` (OpenWrt >= 25.x, `apk`) для 5 архитектур (`mips`, `mipsle`, `armv7`, `arm64`, `x86_64`) + `all`/`noarch` для LuCI.
  - All-in-one пакеты `sirin-full` для установки клиента и веб-интерфейса в один клик.
  - Официальные Makefiles для OpenWrt SDK (`openwrt/sirin-client/Makefile`, `openwrt/luci-app-sirin/Makefile`).
- [x] **v1.6.0–v1.6.2**:
  - **Встроенный In-App Updater в LuCI**: проверка релизов на GitHub и обновление в один клик прямо из браузера.
  - **Аварийный резервный канал Whitelist Fallback (WebRTC TURN VK Calls / DION / S3)** для обхода шатдаунов и белых списков.
  - **Резервный канал Dead Drop (Яндекс Диск / S3-совместимые хранилища)** с энергонезависимым кэшированием параметров.
  - **OpSec & Network Stack Hardening (по методике Vektor T13)**: нормализация TTL=64, TCP MSS Clamping, перехват DNS :53 и блокировка DoT :853.
  - **Принудительный VPN (Force VPN)** по категориям сервисов (AI & LLM, Google Ecosystem, CAD, Streaming) через `dnsmasq nftset`.
- [x] **v1.7.0**:
  - **Интеграция нативного движка Zapret 2 + Lazy On-Demand VPS VPN**:
    - Многоступенчатая адаптивная десинхронизация ТСПУ (`tlsrec_sni_split`, `TCP Urgent MSG_OOB`, `Fake Low-TTL ClientHello` через raw socket с битым TCP checksum, `3-way MultiSplit`, `UDP Fake Burst`) с автообучением на 6 часов.
    - Встроенная поддержка YouTube, Instagram / Meta, Discord (голос + чат) и X / Twitter без нагрузки на зарубежный VPS (`0 Байт` в покое).
    - Мгновенный автоматический перехват в защищённый VPN-туннель при блокировке IP-адреса или при обращении к гео-заблокированным сервисам (ChatGPT, Claude, Gemini, Spotify, Autodesk).
- [x] **v1.8.0**:
  - **Новый интерфейс LuCI 2.0 (4 вкладки + Простой/Экспертный режим)**:
    - Готовые сценарии работы в 1 клик («Умный режим», «Игры и Стриминг», «Максимальная приватность», «Экстренная связь»).
    - Управление категориями зарубежных сервисов (Force VPN: ChatGPT, Claude, Gemini, Spotify, Autodesk) и двумя простыми колонками пользовательских сайтов.
    - Наглядный «Светофор направлений» для самодиагностики и единая панель «Сохранить и применить».
- [ ] **v1.9.0** *(В планах)*:
  - Мультиязычная локализация LuCI UI (gettext `po/ru`, `po/en`).
  - Графики потребления трафика по интерфейсам `sirin0` и `wan` в LuCI.

### 2. Android Client
- [x] **v1.0.0–v1.1.0**: Базовая работа с TUN, защита от утечек IPv6, интеграция с Telegram-ботом.
- [x] **v1.2.0**: Поддержка динамической ротации пула SNI (VK CDN, Кинопоиск, WB, IKEA, Microsoft).
- [x] **v1.3.0–v1.3.5**: Полный редизайн интерфейса в кибер-стиле («Крипто-защита», Cyber-Neon UI), перехват `sirin://` deep-links, GPG-верификация обновлений.
- [x] **v1.6.0–v1.6.2**:
  - Интеграция XHTTP+Reality, Dead Drop (Яндекс Диск / S3), Whitelist Fallback (VK Calls / DION WebRTC TURN) и встроенных резервных ступеней `DEFAULT_NODES`.
- [x] **v1.7.0**:
  - **Гибридный контур Zapret 2 + Lazy On-Demand VPN (`Z2`)**:
    - Пользовательский перехватчик `tunZapretInterceptor` поверх Android `VpnService` (прямой просмотр YouTube, Instagram, Discord через адаптивную десинхронизацию DPI без расхода трафика VPS).
    - Спящий режим VPS-туннеля (0 Байт холостого трафика) с автоматическим пробуждением для ChatGPT, Claude, Gemini, Spotify и заблокированных по IP ресурсов.
- [ ] **v1.8.0** *(В планах)*:
  - Быстрый переключатель в шторке уведомлений (Quick Settings Tile).
  - Расширенная статистика сработавших стратегий Zapret 2 по доменам на экране диагностики.

### 3. Windows Desktop Client
- [x] **v1.0.0**: Клиент на базе Wails с поддержкой Windows TUN (`wintun.dll`), автозапуска и системного трея.
- [x] **v1.1.0**:
  - RuDirect: встроенные 8641 RU-диапазон RIPE NCC через высокоскоростной Win32 IP Helper API (`CreateIpForwardEntry2`).
  - Раздельное туннелирование по доменам и IP-диапазонам (режимы `Bypass` и `Only`).
- [x] **v1.6.0–v1.6.1**:
  - Встроенный модуль автообновления (**In-App Updater**) с проверкой цифровой целостности PE-заголовка и бесшовной перезагрузкой.
  - Интеграция Dead Drop (Яндекс Диск / S3) и аварийного канала Whitelist Fallback (VK Calls / DION TURN).
- [x] **v1.7.0**:
  - **Встроенный движок Zapret 2 + Lazy On-Demand VPS VPN**: прямая десинхронизация ТСПУ через защищённые WAN-сокеты (`IP_UNICAST_IF`) со спящим криптотуннелем и авто-пробуждением для Geo-AI сервисов.
  - Обновлённая иконка приложения Сирин Cyber-Neon.
- [ ] **v1.8.0** *(В планах)*:
  - Регистрация системного URI-протокола `sirin://` в Windows для импорта ключей в один клик.

### 4. Apple iOS / iPadOS Client
- [x] **Spike & Архитектура**: Завершено исследование сетевого стека iOS ([`docs/SPIKE_IOS.md`](SPIKE_IOS.md)).
- [ ] **Фаза 1 (Q1 2027)**: Packet Tunnel Provider на базе Swift + Go Mobile (XCFramework) с поддержкой `zapret` и `VLESS/Noise`.
- [ ] **Фаза 2 (Q2 2027)**: Интерфейс на SwiftUI и распространение через TestFlight.

### 5. Server & Bot Infrastructure
- [x] Автосинхронизация ключей между Telegram-ботом и VPN-демоном `sirind`.
- [x] Ротация SNI и многопутевой мониторинг (Watchdog RU + DE + NL).
- [ ] Интеграция биллинга и платежных шлюзов (Telegram Stars, криптовалюта, СБП).
- [ ] Автоматические уведомления пользователей об окончании срока действия подписки.

---

## 📌 Как предложить идею или повлиять на приоритеты

Вы можете предложить новую функцию или проголосовать за существующие предложения:
- [Обсуждения в GitHub Discussions (Ideas)](https://github.com/goodbrat-lab/sirin-vpn/discussions/categories/ideas)
- [Оформить Feature Request](https://github.com/goodbrat-lab/sirin-vpn/issues/new?template=feature_request.yml)
