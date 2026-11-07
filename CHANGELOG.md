# Changelog

## [router-v1.6.0 / desktop-v1.6.0] - 2026-11-07

### Added
- **Встроенное обновление прямо из интерфейса (In-App Updater для Роутера и Windows Desktop)**:
  - **OpenWrt Router (`luci-app-sirin` / `rpcd/sirin`)**:
    - Интеграция с GitHub Releases API (`goodbrat-lab/sirin-vpn/releases` и `/releases/latest`).
    - Автоматическое определение пакетного менеджера (`apk` для OpenWrt 25.x/Snapshot; `opkg` для OpenWrt 21.x-24.x).
    - Автоматическое определение архитектуры роутера (`mips`, `mipsle`, `armv7`, `arm64`, `x86_64`) с проверкой endianness в `/proc/cpuinfo`.
    - RPC методы `check_update` (проверка версии и доступных пакетов) и `upgrade` (безопасное скачивание в `/tmp`, проверка целостности и размера, установка через `opkg install --force-reinstall` или `apk add --allow-untrusted`, автоматический перезапуск службы).
    - Разрешения в rpcd ACL (`/usr/share/rpcd/acl.d/luci-app-sirin.json`).
    - Интерактивный UI в веб-интерфейсе LuCI (`overview.js`): кнопка «Проверить обновления», динамический бейдж статуса («У вас последняя версия» / «Доступно обновление»), модальное окно с деталями релиза и примечаниями к выпуску, безопасный запуск обновления с индикатором прогресса и автоматической перезагрузкой страницы.
  - **Windows Desktop Client (`apps/desktop`)**:
    - Модуль обновления `updater.go`: функция `CheckDesktopUpdate` для запроса GitHub Releases API, фильтрации черновиков, семантического сравнения версий и сопоставления исполняемых файлов `.exe`.
    - Механизм безопасной самозамены исполняемого файла на Windows (`ApplyDesktopUpdate`): загрузка в `%APPDATA%\Sirin\updates\sirin_update.exe`, валидация PE-заголовка (`MZ`), генерация изолированного batch-хелпера `apply_update.cmd`, который ожидает завершения текущего процесса по PID, атомарно заменяет `.exe`, перезапускает обновленное приложение и удаляет временные файлы.
    - Методы Go-бэкенда Wails: `CheckForUpdate()`, `InstallUpdate()`, `GetAppVersion()`.
    - Обновленный UI диалога настроек (`index.html`, `style.css`, `main.ts`): отображение текущей версии, кнопка «Проверить», блок информации о новом релизе, показ changelog и кнопка «Обновить сейчас».
    - Модульные тесты `TestIsNewerVersion` в `updater_test.go`.

- **Аварийный резервный канал Whitelist Fallback (WebRTC TURN VK Calls / DION / S3)**:
  - **Обход тотальных блокировок и изоляции внешнего интернета (Чебурнет / Белые списки)**:
    - При блокировке всех зарубежных IP и протоколов (UDP Noise, VLESS, XHTTP) клиент автоматически переключается на последнюю ступень лестницы — отечественные WebRTC TURN серверы (VKontakte / Mail.ru / Odnoklassniki / DION).
    - Трафик туннеля маскируется под легитимный медиа-поток отечественных видеозвонков и направляется на доверенные российские IP-адреса из официального белого списка (Минцифры / Роскомнадзор).
  - **Анонимная добыча WebRTC TURN токенов (Zero-Account Harvesting)**:
    - Клиент на лету анонимно обменивает ссылки или хеши групповых звонков VK (`vk.com/call/join/...`) на одноразовые TURN-токены через цепочку API VK Calls (`api.vk.me`, `calls.okcdn.ru`) без авторизации в учетной записи.
    - Поддержка комнат корпоративных видеоконференций DION (`dion.vc/event/...` / ВТБ) с удержанием WebSocket heartbeat в качестве второго независимого источника TURN-релеев.
  - **Режимы работы лестницы**:
    - `auto` (по умолчанию): автоматический fallback — ступень WebRTC релея добавляется в конец списка ступеней и активируется только при отказе прямого UDP/VLESS/XHTTP.
    - `always`: принудительный режим работы исключительно через разрешенные отечественные WebRTC релеи для тестирования и работы в изолированных сетях.
  - **Интеграция в LuCI Web UI (`luci-app-sirin`) и RPC (`rpcd/sirin`)**:
    - Новый раздел «5. Аварийный обход (Whitelist Fallback: WebRTC TURN VK / DION)»: переключатели активации, выбора режима Auto/Always, ввод ссылок/хешей VK Calls и комнат DION, настройка резервного TURN-сервера.
    - RPC методы `get_whitelist_config` и `set_whitelist_config`.
  - **Интеграция в Windows Desktop Client (`apps/desktop`)**:
    - Поддержка `WhitelistFallback` в локальной конфигурации `%APPDATA%/Sirin/config.json` и в `buildLadder`.
  - **Тесты**:
    - `TestCleanVKHash`, `TestCleanDionSlug`, `TestBuildFromParams_WhitelistAuto`, `TestBuildFromParams_WhitelistAlways`, `TestBuildFromStatic_RelayEntry`, `TestDesktop_BuildLadder_WhitelistFallback`.
- **Встроенный модуль Anti-DPI (обход замедления YouTube и Discord без VPN)**:
  - **Прямой просмотр YouTube 4K/8K и Discord Voice/Gateway на скорости WAN**:
    - Трафик целевых CDN-нод (`*.googlevideo.com`, `*.ytimg.com`, `discord.com`, `*.discord.gg`) не заворачивается в заграничный VPN-туннель, а идет напрямую через провайдера с минимальным пингом и 0 расходом серверного трафика.
  - **Прозрачный локальный прокси с фрагментацией TLS ClientHello**:
    - Встроенный чистый Go-сервер (`apps/router/antidpi.go`), слушающий на порту `127.0.0.1:56100` через nftables `redirect`.
    - Парсер ClientHello по спецификациям TLS 1.2/1.3 (RFC 5246 / RFC 8446) с извлечением Server Name Indication (SNI).
    - Фрагментация первого пакета TLS ClientHello на два TCP-сегмента (по умолчанию на позиции byte 2) с включенным `TCP_NODELAY` и микрозадержкой между сегментами, что разрушает сигнатуру в фильтрах ТСПУ/DPI без возможности потоковой склейки на лету.
    - Определение оригинального адреса назначения через `getsockopt(SO_ORIGINAL_DST)` (`linux`), с архитектурной изоляцией других ОС через build tags.
  - **Принудительный откат с QUIC на TCP (QUIC Reject)**:
    - Блокировка пакетов UDP 443 для доменов YouTube (`ip daddr @antidpi_cidrs udp dport 443 reject`), что мгновенно форсирует переключение браузеров и мобильных устройств на защищаемый TCP HTTP/2 в течение 5-10 мс.
  - **Интеграция с dnsmasq и динамическими nftables ipsets**:
    - Автоматическая генерация `/tmp/dnsmasq.d/sirin-antidpi.conf` с директивами `nftset=/domain/4#sirin#antidpi_cidrs`.
    - По мере DNS-резолвинга IP-адреса YouTube и Discord автоматически попадают в ядерный сет `antidpi_cidrs` с таймаутом жизни.
  - **Интеграция в LuCI Web UI (`luci-app-sirin`) и RPC (`rpcd/sirin`)**:
    - Новый раздел «4. Обход замедления без VPN (Anti-DPI: YouTube / Discord)»: чекбоксы YouTube/Discord/QUIC Reject, настройка позиции сплита, пользовательские домены, локальный порт прокси.
    - RPC методы `get_antidpi_config` и `set_antidpi_config`.
  - **Тесты**:
    - `TestExtractSNI`, `TestExtractSNI_InvalidData`, `TestAntiDPIConfig_GetTargetDomains`, `TestFragmentAndSend`, `TestGenerateNftablesRules_AntiDPI`, `TestConfig_AntiDPIToml`.
- **Резервный канал Dead Drop (Яндекс Диск / S3-совместимые хранилища)**:
  - **Обход блокировок центрального API**:
    - При недоступности основного сервера конфигурации (`/v1/params`) или блокировке его домена/IP по ТСПУ клиент автоматически опрашивает резервные источники Dead Drop (`dead_drop_urls`).
    - Полная поддержка публичных ссылок Яндекс Диска (`https://disk.yandex.ru/d/...`, `https://yadi.sk/d/...`): автоматический двухшаговый резолвинг прямой ссылки через официальный открытый REST API `cloud-api.yandex.net` без авторизации.
    - Поддержка прямых ссылок на S3-совместимые хранилища (VK Cloud Object Storage, Yandex Cloud Object Storage, Hotbox).
  - **Локальное энергонезависимое кеширование (Zero-Downtime Resilience)**:
    - При любом успешном получении конфигурации (от API или Dead Drop) актуальный список узлов сохраняется в локальный кеш (`/tmp/sirin_params_cache.json` в ramfs на роутере; `%APPDATA%/Sirin/params_cache.json` на Windows Desktop).
    - При повторных перезагрузках или полном отключении интернета клиент способен мгновенно подняться из кеша до попытки статического fallback.
  - **Интеграция в LuCI Web UI и RPC**:
    - Новое многострочное поле ввода резервных ссылок Dead Drop в веб-интерфейсе LuCI (`overview.js`), валидация и сохранение в `/etc/sirin/client.toml` через `rpcd/sirin`.
  - **Тесты**:
    - Модульные тесты `TestIsYandexDiskLink`, `TestFetchDeadDrop_DirectS3`, `TestFetchDeadDrop_CachePersistence`, `TestBuildLadder_DeadDropFallback`.
- **OpSec & Network Stack Hardening (по методике Vektor T13)**:
  - **Нормализация TTL (TTL Normalization)**:
    - Маскировка маршрутизатора и раздачи интернета клиентами LAN. Для всех исходящих пакетов через туннель `sirin0` в цепочке `postrouting_mangle` устанавливается фиксированный TTL (по умолчанию 64 для Linux/Android/iOS/macOS; 128 для Windows).
    - Предотвращает обнаружение промежуточных транзитных хопов и анализ p0f/OS fingerprinting на стороне ТСПУ и целевых серверов.
  - **TCP MSS Clamping (выравнивание MTU/MSS)**:
    - Автоматическая подгонка MSS под MTU туннеля (`tcp flags syn tcp option maxseg size set rt mtu` в nftables и `--clamp-mss-to-pmtu` в iptables). Исключает фрагментацию и застревание пакетов ("черные дыры" MTU).
  - **Anti-DNS-Leak (принудительный перехват DNS на роутере)**:
    - Цепочка `chain dstnat` перехватывает все клиентские DNS-запросы на порту 53 UDP/TCP (`iifname "br-lan" redirect to :53`) и направляет их в локальный защищенный dnsmasq, блокируя нежелательные прямые запросы к 8.8.8.8, 1.1.1.1 и умным IoT-устройствам.
  - **Блокировка DNS-over-TLS (DoT Port 853)**:
    - Запрет трафика на порт 853 (`reject`) в цепочке `forward`, предотвращающий скрытый обход локального DNS мобильными ОС (Android Private DNS / DoT).
  - **Поддержка iptables fallback**:
    - Зеркальная поддержка OpSec-правил при fallback на устаревшие прошивки без nftables.
  - **Интеграция в LuCI Web UI (`luci-app-sirin`)**:
    - Переключатели нормализации TTL, целевого TTL, перехвата DNS и блокировки DoT в секции настроек с валидацией, сохранением и перезапуском через `rpcd/sirin`.
  - **Тесты**:
    - `TestConfig_OpSecDefaults`, `TestConfig_OpSecToml`, `TestGenerateNftablesRules_OpSec`.

## [desktop-v1.1.0] - 2026-11-06

### Added
- **RuDirect (8641 CIDR из реестра RIPE NCC)**:
  - Прямой доступ к российским сайтам и сервисам (Госуслуги, банковские приложения, сервисы Яндекса, VK) на максимальной скорости домашнего провайдера без VPN-задержек и блокировок по геолокации.
  - Высокопроизводительная установка маршрутов через Win32 IP Helper API (`CreateIpForwardEntry2` / `DeleteIpForwardEntry2` из `iphlpapi.dll`) за < 150 мс для всех 8641 диапазонов вместо минутного перебора через `route.exe`.
  - База подсетей `ru-ranges.txt` встроена непосредственно в исполняемый файл через `//go:embed`.
- **Раздельное туннелирование по доменам и IP-диапазонам (Split Tunneling)**:
  - Режим `Bypass` (по умолчанию): весь трафик защищен Wintun VPN, а трафик RuDirect и указанные пользователем домены и CIDR идут напрямую через физический шлюз.
  - Режим `Only`: в VPN-туннель направляются исключительно указанные домены и IP-диапазоны, остальной трафик не затрагивается.
  - Резолвинг доменов в IPv4-адреса и динамическая установка `/32` маршрутов.
  - Автоопределение физического интерфейса и шлюза по умолчанию (`GetPhysicalDefaultGateway`).
  - Чистый откат и удаление установленных маршрутов при отключении туннеля (`route.Cleanup()`).
- **Интерфейс настроек Desktop (Wails v3 UI)**:
  - Переключатель RuDirect с пояснением.
  - Выбор режима туннелирования (`Bypass` vs `Only`).
  - Многострочные поля ввода пользовательских доменов и CIDR с валидацией.
  - Интеграция с локальной конфигурацией `%APPDATA%/Sirin/config.json` и Wails v3 API (`GetRoutingConfig`, `SaveRoutingConfig`).
- **Тесты**:
  - Модульные тесты `TestParseCIDR`, `TestGetRuCIDRs`, `TestResolveDomains`, `TestFindPhysicalGateway`.


## [router-v1.5.0] - 2026-11-06

### Added
- **Сборка и упаковка в форматы OpenWrt `.ipk` (opkg) и `.apk` (OpenWrt 25.x)**:
  - Автономный сборщик пакетов на чистом Go (`apps/router/packager/main.go`), не требующий установки OpenWrt SDK: генерирует валидные архивы со строгими правами файлов (0755/0644) и владельцем `root:root`.
  - Отдельные пакеты `sirin-client` и универсальный архитектурно-независимый пакет `luci-app-sirin` (`all`/`noarch`).
  - Комплексные All-in-one пакеты `sirin-full` (демон + LuCI в одном файле), устанавливаемые одной командой `opkg install` или `apk add`.
  - Пакеты собраны под все 5 поддерживаемых процессорных архитектур роутеров: `mips`, `mipsle`, `armv7`, `arm64`, `x86_64`.
  - Добавлены Makefiles для сборки через официальный OpenWrt SDK (`openwrt/sirin-client/Makefile` и `openwrt/luci-app-sirin/Makefile`).
  - Добавлена подробная документация [`docs/OPENWRT_PACKAGES.md`](docs/OPENWRT_PACKAGES.md) с пошаговыми инструкциями для OpenWrt 21–24.x (`opkg`) и OpenWrt 25.x (`apk`).
  - Unit-тесты верификации целостности архивов `TestVerifyIPK` и `TestVerifyAPK`.

## [router-v1.4.0] - 2026-11-05

### Added
- **Раздельное туннелирование по портам (Port-Based Split Tunneling)**:
  - Поддержка одиночных портов (`80`, `443`), диапазонов (`8080-8090`, `27000-27050`) и явного указания протокола (`tcp:80`, `udp:53`, `udp:27000-27050`).
  - Режим `port_mode = "only"`: через VPN направляется только трафик на указанные порты (например, веб-порты 80 и 443), а все остальные порты (игры, торренты) направляются напрямую в WAN через физический шлюз провайдера.
  - Режим `port_mode = "bypass"` (Gaming Bypass): указанные порты (например, порты Steam, Valorant, Riot, EA) принудительно направляются напрямую в WAN с минимальным нативным пингом, а весь остальной трафик идет через защищенный туннель.
  - Атомарная генерация наборов nftables (`set split_ports_tcp` и `set split_ports_udp` с типом `inet_service` и флагом `interval`).
  - Полная поддержка в веб-интерфейсе LuCI (`luci-app-sirin`): радиокнопки режима, многострочный ввод портов с клиентской валидацией, сохранение и перезапуск демона через `rpcd/sirin`.
  - Модульные тесты `TestValidatePort`, `TestNormalizePortMode`, `TestGetSplitPorts`, `TestGenerateNftablesRules_WithPorts_WebOnly`, `TestGenerateNftablesRules_WithPorts_GamingBypass`, `TestSplitTunnelConfig_PortsFromTOML`.

## [server-v1.5.0] - 2026-10-27

### Added
- **Диагностический движок соединения (`server/internal/bot/diagnostics/`)**:
  - Комплексные проверки: статус ключа, доступность API `/health` (с изоляцией счетчиков для клиентов, S-06), получение параметров `/v1/params`, TCP probe до релея с замером RTT, проверка прохождения туннеля через `ip.me` и динамическая сверка версии приложения с GitHub API.
  - Безопасные авто-фиксы: сброс на Direct NL при сбое/перегрузке релея, привязка аккаунта Telegram, ссылка на обновление клиента.
  - Каскадная защита: при временных таймаутах пробы авто-фикс не сбрасывает клиентов каскадно, предлагая ручной выбор.
- **Архитектура Multi-Device (`bot_users.json`)**:
  - Поддержка произвольного числа устройств (`devices[]`) на одного пользователя: UUID, метка, платформа, транспорт, версия, даты добавления и диагностики.
  - Бесшовная автомиграция одиночных UUID из старого формата `bot_users.json`.
  - Выбор устройства в диагностике и раздельное хранение ключей в `/mykey` с возможностью добавления новых устройств.
  - Отображение списка устройств в админ-карточке клиента.
- **Система тикетов технической поддержки (`server/internal/bot/tickets/`)**:
  - Хранение в `/etc/sirin/tickets.json` с правами `0600`, исключением из git и автоматической очисткой закрытых обращений старше 90 дней.
  - Защита OpSec: тексты тикетов не выводятся в stdout (только метаданные), маскирование UUID и IP в интерфейсе.
  - Защита от спама: лимит не более 3 открытых тикетов, не более 1 тикета в час, не более 5 сообщений за 10 минут.
  - Эскалация из диагностики: автозаполнение контекста устройства, категории и снимка проверок.
  - Панель тикетов у администратора: карточка обращения, двусторонняя переписка, смена статусов, уведомления в Telegram.
  - Раздел «📋 Мои обращения» у клиента.
- **База знаний (FAQ) (`server/cmd/sirinbot/menu/faq.go`)**:
  - 6 категорий: не подключается, сайты не открываются, низкая скорость, проблемы с ключом, раздельное туннелирование, OpenWrt-роутер.
  - Кнопки «Помогло» и «Не помогло → Запустить диагностику» со сквозной передачей темы.
- **Проактивный Lazy-мониторинг (`server/cmd/sirinbot/menu/monitoring.go`)**:
  - Проверка при `/start`: предупреждения об окончании ключа (< 3 дней) и перегрузке релеев с защитой от дублей (не чаще 1 раза в сутки на устройство).
  - Дайджест мониторинга для администратора в `/menu`.

### Security
- Добавление `*tickets.json*` и `*bot_users.json*` в `.gitignore`.
- Строгие права `0600` на файл тикетов и бэкапы.
- Фиксация отсутствия сетевого ограничения `device_id` в ядре `sirind` в `docs/KNOWN_ISSUES.md`.

## [android-v1.3.5] - 2026-10-24

### Security
- Переход на production keystore (новый сертификат RSA 4096-bit)
- CI: fail-fast при отсутствии keystore secrets
- Production keystore вместо debug.keystore

### Breaking
- Требуется однократное удаление старой версии (1.3.4 и ранее) и установка 1.3.5 начисто — из-за смены сертификата подписи.

### Fixed
- versionCode = 11, versionName = "1.3.5"

## [server-v1.4.0] - 2026-10-24

### Added
- **Единое хранилище релеев (`relays.json`)**:
  - Динамическая конфигурация произвольного числа релеев (`0..N`) без перезапуска сервисов.
  - Атомарное сохранение с автоматическим созданием резервных копий (`.bak.<ts>`).
  - Потокобезопасное кэширование и подсчёт активных клиентов.
- **Интерактивный раздел «📡 Релеи» в Telegram-боте (`sirinbot`)**:
  - Экран списка релеев с отображением загрузки (`👥 X / Y`, остаток мест).
  - Карточка релея: изменение лимита, смена хоста, включение/отключение и удаление (при 0 клиентов).
  - Защищённое отключение узла с подтверждением и автопереводом клиентов на Direct NL с уведомлениями в Telegram.
  - Пошаговый FSM-мастер добавления нового релея с автогенерацией ID.
  - Выбор узла из карточки клиента с контролем емкости (`max_clients`).
- **Расширение API `/v1/params` (`sirinapi`)**:
  - Динамическая отдача адреса и портов релея по `relay_id` клиента.
  - Передача HTTP-заголовка `X-Relay-Load: <занято>/<лимит>`.
  - Безопасный fallback на Direct NL при отсутствии узла или отключённом статусе.

## [router-v1.3.1] - 2026-10-22

### Fixed
- XHTTP: transXHTTP теперь обрабатывается как стриминговый транспорт (не UDP), устранён fallback на VLESS
- XHTTP: путь без trailing slash (совместимость с Xray)

## [router-v1.3.0] - 2026-10-22

### Added
- XHTTP+Reality транспорт (P1): полнодуплексный HTTP/2 поверх Reality, маскировка под Chrome
- Path и Mode в ladder, поддержка из /v1/params и статического client.toml

### Verified
- E2E: клиент → :8443 → sirind:56006

## [android-v1.3.4] - 2026-10-22

### Fixed
- **Исправлена инициализация криптопровайдера Bouncy Castle на Android**:
  - Устранена коллизия со встроенным системным урезанным провайдером Android `com.android.org.bouncycastle`.
  - Явная передача экземпляра `BouncyCastleProvider` в `JcaPGPContentVerifierBuilderProvider` и `JcaKeyFingerprintCalculator`.
  - Устранена ошибка ложного отклонения GPG-подписи в `UpdateChecker`.

## [android-v1.3.3] - 2026-10-22

### Changed
- **Повышение версии до 1.3.3 (versionCode 9)**:
  - Позволяет установленным клиентам `1.3.2-direct` автоматически обнаружить и применить обновление через встроенный `UpdateChecker`.
  - Пакет подписан и проверен через трёхуровневую цепочку доверия (GPG `checksums.txt.asc` -> SHA-256 -> Android Keystore).

## [android-v1.3.2] - 2026-10-22

### Security
- **GPG-подпись checksums.txt (защита от подмены релизов на GitHub)**:
  - CI генерирует отсоединённую цифровую подпись `checksums.txt.asc` с использованием RSA 4096-bit GPG ключа автора.
  - В APK вшит открытый ключ `sirin_pubkey.asc`.
- **Трёхуровневая эшелонированная верификация обновлений**:
  - `UpdateChecker` проверяет подлинность `checksums.txt` через OpenPGP (BouncyCastle).
  - Сверка SHA-256 хэша скачанного APK.
  - Сверка сертификата подписи APK с текущим установленным приложением через Android `PackageManager`.
  - При любом несоответствии скачанный файл немедленно удаляется из файловой системы.
- **Kill Switch совместим со split tunneling (BYPASS)**:
  - Проверено и подтверждено: заглушка «чёрной дыры» Kill Switch сохраняет исключение пакетов (`addDisallowedApplication`), позволяя bypass-трафику безопасно продолжать идти напрямую через WAN.

### Documentation
- `docs/ACCEPTANCE_ANDROID.md`: детальный пошаговый чек-лист ручной приёмки на реальном устройстве (Kill Switch, GPG, IPv6, Split Tunneling).
- `docs/SECURITY.md`: архитектура цепочки доверия и модель угроз автообновления.

## [router-v1.2.0] - 2026-10-21

### Added
- **Configurable Split Tunneling (Bypass / Only)**:
  - Режим `bypass`: весь интернет направляется через VPN, за исключением адресов из split-списка (направляются через WAN).
  - Режим `only`: через VPN направляются ТОЛЬКО адреса и домены из split-списка, весь остальной трафик идёт напрямую через WAN.
  - Поддержка произвольных IPv4 CIDR и доменных имен в секции `[split_tunnel]` конфигурационного файла `client.toml`.
  - Встроенный список RuDirect (8641 CIDR РФ) как часть bypass-списка по умолчанию.
  - В режиме `only` параметр `use_ru_direct` безопасно игнорируется с предупреждением в системном логе.
- **Маршрутизация на базе nftables sets и policy routing**:
  - Таблицы маршрутизации 100 (VPN) и 200 (WAN) с fwmark `0x1` и `0x2`.
  - Атомарная загрузка интервальных множеств `local_cidrs` и `split_cidrs` через nftables (`flags interval`).
  - Быстрый алгоритм дедупликации и слияния подсетей `mergeCIDRs` для исключения коллизий интервалов в ядре Linux.
  - Обновлён сетевой протектор Linux: марка `0x2` и правило `meta mark 0x2 return` в цепочке output исключают зацикливание сокетов VPN-движка.
  - Полный идемпотентный сброс всех правил nftables, ip rule и таблиц при остановке (`cleanupRouting`).
- **Управление в веб-интерфейсе LuCI**:
  - Карточка «3. Раздельное туннелирование» с переключателями режимов Bypass/Only, чекбоксом RuDirect, текстовыми областями для CIDR и доменов.
  - Строгая валидация форматов CIDR и доменных имен на клиенте (JS) и сервере (`rpcd/sirin`).
  - Методы ubus `get_split_config`, `set_split_config`, `get_ru_direct_info` с разграничением прав в ACL.
  - Обновлена карточка диагностики с отображением активного режима split tunneling.

### Compatibility
- Полная обратная совместимость со старыми конфигурациями: при отсутствии секции `[split_tunnel]` клиент работает в режиме `bypass` со встроенным RuDirect.
- Статический резолв доменов при старте клиента (IPv6 не туннелируется, только IPv4).

## [router-v1.1.1] - 2026-10-18

### Fixed
- **Критический баг маскирования пароля в LuCI Web UI**: исправлено извлечение существующего ключа в RPCD `match($0, /"[^"]*"/)`, предотвращающее перезапись пароля маской `69c8e325…` при сохранении формы
- Исправлено создание родительской директории конфига в `rpcd/sirin` через `dirname "$CONFIG_FILE"`

### Added
- Строгая валидация формата пароля (UUID v4) на бэкенде RPCD и динамическое предупреждение в JS-интерфейсе LuCI
- Полная адаптивная верстка (responsive) для мобильных экранов (до 375px) с поддержкой тем LuCI (`bootstrap`, `openwrt-2020`, `argon`)

## [router-v1.1.0] - 2026-10-18

### Added
- LuCI-приложение luci-app-sirin (веб-UI): статус, настройки, диагностика, логи
- install.sh: флаги --with-ui / --no-ui, авто-детект LuCI
- RPCD-бэкенд /usr/libexec/rpcd/sirin (ubus object «sirin»)

## [router-v1.0.1] - 2026-10-18

### Fixed
- install.sh: корректная обработка отсутствующего client.toml.example
- Синхронизирован состав архива с install.sh (шаблон конфигурации добавлен как в корень архива, так и в configs/)

## [1.3.0] - 2026-10-21

### Added
- **UI/UX Redesign — Стиль «Крипто-защита» (Dark Material & Cyber-Neon)**:
  - Палитра: глубокий темный фон `#0B0C10`, карточки `#151623`, акцент Cyber Blue (`#00FFFF`) → Neon Purple (`#8A2BE2`), ошибки `#FF3366`, рукопожатие `#FFB300`
  - Интерактивный центральный логотип Сирин с кокошником, крыльями, цифровым замком, цепями и кибер-кубами
  - 4-позиционная машина состояний (State Machine): Отключено, Подключение, Подключено, Ошибка
  - Анимация поднимающихся пакетов данных из-под кнопки питания к логотипу
  - Круглая интерактивная кнопка питания (Power Button) с breathing-пульсацией и неоновым сине-пурпурным свечением
  - Плашки настроек со скруглением 14dp и 1px градиентной обводкой при активных свитчах
  - Тонкие контурные иконки меню (Line Art) с градиентным штрихом
  - Минималистичное строго вертикальное монохромное перо в статус-баре (`ic_sirin_notify.xml`)
  - Material Design уведомление с бирюзовым акцентом и нативной кнопкой «ОТКЛЮЧИТЬ»

## [router-v1.0.0] - 2026-10-18

### Added
- Router-клиент для OpenWrt 24.x/25.x (MIPS, MIPSLE, ARMv7, ARM64, x86_64)
- Split tunneling (RuDirect: 4945 CIDR через WAN)
- Full tunnel через TUN sirin0 (0.0.0.0/1, 128.0.0.0/1)
- Автоматический скрипт установки `install.sh` и init-скрипт procd

## [1.2.0] - 2026-10-18

### Added
- SNI rotation: 4 domains (sun8-21.userapi.com, kinopoisk.ru, wildberries.ru, ikea.com). Auto-fallback if one blocked.
- Testbench scenario: exact SNI blocking via NFQUEUE

### Resilience
- Before: 1 SNI (sun8-21) + UDP = 2 points of failure
- After: 4 SNI + UDP = 5 points of failure
- Verified: block sun8-21 -> switch to kinopoisk in 4.5s

### Infrastructure
- Xray serverNames: 4 domains
- API /v1/params: SNIs[] array
- Android client: parses snis, passes to engine

## [1.1.1] - 2026-10-17

### Added
- Deep-link support: https://crp.guseynovia.ru/key?uuid=... opens app
- "Change key" button in Settings with immediate reconnect
- Telegram bot: HTTPS link + inline button "Open in Sirin"
- KeyIcon in brand style

### Fixed
- APK v1.1.0 was missing deep-link and change-key features (not committed)
- v1.1.1 includes all previous features + new ones

## [1.1.0] - 2026-10-17

Automatic client key synchronization between clients.json and sirind.

### Added
- Auto-sync: `sirind` reads allowed UUIDs directly from `/etc/sirin/clients.json` with fallback to `sirind.toml`
- Background sync loop: checks `clients.json` mtime every 30 seconds
- Thread-safe key handling via `sync.RWMutex`
- Key caching: avoids re-deriving X25519 wrap keys for unchanged UUIDs
- Unit test suite for client synchronization (`sync_test.go`)
- Support for generating and using keys from Telegram bot without restarting `sirind`

### Changed
- `sirind` container mounts `/etc/sirin` directory as read-only (`ro`)
- Configuration accepts optional `clients_path` parameter

## [1.0.1] - 2026-10-17

Working release with production configuration via GitHub Secrets.

### Changed
- CI: use `SIRIN_CONFIG_KT` secret for release builds to produce production-ready APK
- Fixed `version.properties` formatting (versionCode: 2, versionName: 1.0.1)

## [1.0.0] - 2026-10-17

First public release of Sirin VPN.

### Features
- Android client with Sirin Feather visual element
- Own server: UDP Noise NK + TCP VLESS Reality (XTLS Vision)
- Automatic fallback if UDP is blocked (392ms)
- Split tunneling (RU direct, rest via VPN)
- IPv6 leak protection (fd00::1/128 on tun)
- API with HTTPS (Let's Encrypt)
- Telegram bot for key management (@sirin_crp_bot)
- SNI masking via VK CDN (sun8-21.userapi.com)
- Docker deployment (docker-compose.yml)

### Fixed
- IPv6 leak (critical: was leaking through Wi-Fi)
- MTU mismatch (1280 vs 1100, caused crashes)
- Split tunneling trap (ONLY mode with 1 app blocked everything)

### Tech Stack
- Go 1.27 (engine, server, API, bot)
- Kotlin 2.x + Jetpack Compose (Android)
- Docker + docker-compose (deployment)
- Xray-core (VLESS Reality)

### Based on
- Meridian VPN (JinComputers) - MIT License
- See NOTICE for full credits
