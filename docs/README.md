# Индекс документации Sirin VPN

Добро пожаловать в центр технической документации и руководств пользователя **Sirin VPN**. Здесь собраны все инструкции по установке, настройке, диагностике и разработке компонентов системы.

---

## 🚀 1. Установка и запуск

### 1.1. Android
- **Актуальный релиз:** [sirin-v1.3.1.apk](https://github.com/goodbrat-lab/sirin-vpn/releases/download/v1.3.1/sirin-v1.3.1.apk)
- **Инструкция по установке:**
  1. Скачайте APK-файл на устройство под управлением Android 8.0+.
  2. Разрешите установку из неизвестных источников при первом запуске файла.
  3. Откройте приложение, укажите UUID-ключ (из Telegram-бота `@sirin_crp_bot` или по deep-link ссылке) и нажмите «Подключить».

### 1.2. Windows Desktop
- **Актуальный релиз:** [Sirin-VPN-1.0.0-Setup.exe](https://github.com/goodbrat-lab/sirin-vpn/releases/download/desktop-v1.0.0/Sirin-VPN-1.0.0-Setup.exe)
- **Инструкция по установке:**
  1. Запустите инсталлятор `Setup.exe` от имени администратора.
  2. Завершите стандартную процедуру установки (драйвер Wintun устанавливается автоматически).
  3. В окне приложения введите ключ доступа и активируйте туннель.

### 1.3. Router (OpenWrt)
- **Полное руководство пользователя:** [**`docs/ROUTER.md`**](ROUTER.md) — подробный разбор установки, файрвола, PPPoE, веб-интерфейса LuCI и устранения конфликтов.
- **Быстрая автоустановка в одну команду:**
  ```bash
  wget -O - https://raw.githubusercontent.com/goodbrat-lab/sirin-vpn/master/apps/router/install.sh | sh
  ```
- **Чеклист верификации:** [**`docs/ROUTER_INSTALL_VERIFICATION.md`**](ROUTER_INSTALL_VERIFICATION.md) — пошаговый чеклист проверки работоспособности сервиса, маршрутизации и UI.

---

## ⚙️ 2. Конфигурация и управление

- **Раздельное туннелирование (Split Tunneling):**
  - Описание режимов `bypass` и `only`, настройка CIDR и доменов: см. [Раздел 5 в `ROUTER.md`](ROUTER.md#5-раздельное-туннелирование-split-tunneling).
  - Встроенный список RuDirect (8641 диапазон адресов РФ).
- **Telegram-бот (@sirin_crp_bot):**
  - [**`docs/BOT_COMMANDS.md`**](BOT_COMMANDS.md) — полный список команд администратора и клиента (`/newkey`, `/list`, `/info`, `/extend`, `/disable`, `/enable` и др.).
  - [**`docs/BOT_UX_MAP.md`**](BOT_UX_MAP.md) — карта состояний FSM и логика интерактивных inline-клавиатур.

---

## 🛠️ 3. Диагностика и решение проблем

- [**`docs/KNOWN_ISSUES.md`**](KNOWN_ISSUES.md) — актуальный список известных неполадок и проверенные методы их устранения:
  - Конфликты с прозрачными прокси (TPROXY в sing-box / xray / passwall);
  - Особенности работы с PPPoE-интерфейсами (`pppoe-wan`);
  - Предотвращение утечек IPv6 через режим `--strict-tunnel`.
- [**`docs/HOW_TO_REPORT.md`**](HOW_TO_REPORT.md) — инструкция по сбору журналов (`logread`, `adb logcat`, `/var/log/messages`) и оформлению качественных отчётов об ошибках.
- [**`docs/FEEDBACK.md`**](FEEDBACK.md) — регламент предоставления обратной связи, каналы коммуникации и формы тикетов.
- [**`docs/FEEDBACK_BOARD.md`**](FEEDBACK_BOARD.md) — публичный трекер предложений и отзывов сообщества.

---

## 🔬 4. Разработка, исследования и архитектура

- [**`docs/DECISION_MATRIX.md`**](DECISION_MATRIX.md) — журнал архитектурных решений (ADR): выбор протоколов Noise NK / VLESS, разделение прав в LuCI через RPCD ACL, реализация интервальных списков в nftables.
- [**`docs/SPIKE_IOS.md`**](SPIKE_IOS.md) — техническое исследование реализации клиента для iOS / iPadOS с использованием NetworkExtension и Go Mobile.
- [**`docs/ROADMAP_INVISIBLE_VEIL.md`**](ROADMAP_INVISIBLE_VEIL.md) — план усиления маскировки трафика против сигнатурного анализа ТСПУ.
- [**`docs/ROADMAP.md`**](ROADMAP.md) — общая дорожная карта развития проекта Sirin VPN.

---

## 💬 5. Сообщество и ссылки

- [Форум обсуждений: GitHub Discussions](https://github.com/goodbrat-lab/sirin-vpn/discussions)
- [Интерактивные шаблоны баг-репортов](https://github.com/goodbrat-lab/sirin-vpn/issues/new/choose)
- [Официальный сайт документации GitHub Pages](https://goodbrat-lab.github.io/sirin-vpn/)
