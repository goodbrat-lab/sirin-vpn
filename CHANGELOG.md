# Changelog

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
