<div align="center">
  <img src="docs/assets/logo.png" alt="Sirin VPN Logo" width="140" height="140" />
  <h1>Сирин (Sirin) VPN</h1>
  <p><strong>Гибридный комплекс обхода блокировок ТСПУ РКН: нативный Zapret 2 + Спящий (Lazy On-Demand) Stealth VPN для всей экосистемы устройств</strong></p>

  <p>
    <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/releases/latest"><img src="https://img.shields.io/badge/Release-Latest-brightgreen.svg?logo=github&logoColor=white" alt="Latest Release" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/releases"><img src="https://img.shields.io/badge/Platforms-Android%20%7C%20Windows%20%7C%20OpenWrt-blue.svg" alt="Platforms" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/discussions"><img src="https://img.shields.io/badge/GitHub-Discussions-purple.svg?logo=github" alt="Discussions" /></a>
    <a href="https://goodbrat-lab.github.io/sirin-vpn/"><img src="https://img.shields.io/badge/Docs-GitHub%20Pages-informational.svg?logo=githubpages" alt="GitHub Pages" /></a>
  </p>
</div>

---

## ⚡ О продукте и архитектуре

**Сирин (Sirin) VPN** — независимый клиент-серверный комплекс обхода интернет-цензуры нового поколения, объединяющий **прямую пакетную десинхронизацию ТСПУ (технологии Zapret 2)** и **спящий по требованию (Lazy On-Demand) криптотуннель до зарубежного VPS**.

### Ключевые возможности комплекса:
1. **Нативный движок Zapret 2 (Первичный контур — 0 Байт на VPS):**
   - Для **YouTube (4K/8K)**, **Instagram / Meta**, **Discord (голос и чат)**, **X / Twitter** и большинства сайтов клиент сначала применяет прямое соединение с многоступенчатой десинхронизацией DPI (`tlsrec_sni_split`, `TCP Urgent MSG_OOB`, `Fake ClientHello с малым TTL и битым TCP checksum`, `3-way MultiSplit`, `UDP Fake Burst`).
   - Пока работает прямой обход Zapret 2, VPN-туннель к вашему зарубежному VPS находится в **спящем режиме (0 Байт трафика)** — ТСПУ РКН не видит постоянных шифрованных сессий к вашему серверу и не блокирует IP-адрес VPS.
2. **Бесшовный перехват в Lazy On-Demand VPN (Вторичный контур):**
   - Если ТСПУ провайдера заблокировал ресурс по IP-подсети (и все фазы Zapret 2 не получили ответа) **или** вы открываете сервис с гео-блокировкой со стороны самого зарубежного сайта (**ChatGPT / OpenAI, Claude, Gemini, Grok, Spotify, Autodesk** и др.), клиент автоматически за миллисекунды пробуждает шифрованный VPN-туннель (`VLESS Reality XTLS-Vision`, `XHTTP/2 Reality` или `UDP Noise NK`) и прозрачно перенаправляет этот поток через зарубежный VPS.
   - После периода отсутствия трафика туннель автоматически паркуется обратно в спящий режим.
3. **Раздельное туннелирование (RuDirect 8641 CIDR + Порты):**
   - Встроенная агрегированная база подсетей РФ от RIPE NCC (`8641 CIDR`) и правила по портам (`Gaming Bypass`) обеспечивают прямой доступ к российским банкам, Госуслугам, маркетплейсам и онлайн-играм с минимальным пингом.
4. **Аварийный обход «Белых списков» (VK Calls / DION WebRTC TURN) и Dead Drop:**
   - При тотальной блокировке зарубежных IP на мобильных сетях трафик маскируется под видеозвонки ВКонтакте (`VK Calls`) и корпоративной платформы `DION` через отечественные серверы WebRTC TURN из белого списка Минцифры.
   - При блокировке управляющего домена клиент автоматически загружает свежую конфигурацию через резервные хранилища **Dead Drop** (Яндекс Диск / S3).

---

## 📱 Поддерживаемые платформы

| Платформа | Где скачать | Руководство по установке | Статус |
|:---|:---|:---|:---:|
| **Android** | [📦 Релизы на GitHub (APK)](https://github.com/goodbrat-lab/sirin-vpn/releases) | [📖 Инструкция для Android](docs/README.md#11-android) | ✅ Стабильный |
| **Windows Desktop** | [📦 Релизы на GitHub (Setup.exe)](https://github.com/goodbrat-lab/sirin-vpn/releases) | [📖 Инструкция для Windows](docs/README.md#12-windows-desktop) | ✅ Стабильный |
| **Router (OpenWrt)** | [📦 Архивы и пакеты (.ipk / .apk для 5 архитектур)](https://github.com/goodbrat-lab/sirin-vpn/releases) | [📖 Руководство OpenWrt & LuCI](docs/ROUTER.md) | ✅ Стабильный |
| **iOS / iPadOS** | *В разработке (архитектура NetworkExtension)* | [🔬 Исследование (Spike iOS)](docs/SPIKE_IOS.md) | ⏳ В планах |
| **macOS / Linux** | *Запланировано на базе Wails & core/engine* | [🗺️ Roadmap](docs/ROADMAP.md) | ⏳ В планах |

---

## 🚀 Быстрый старт

### 🤖 Android
1. Загрузите свежий APK-пакет со страницы [релизов проекта на GitHub](https://github.com/goodbrat-lab/sirin-vpn/releases).
2. Запустите приложение и вставьте персональный ключ (UUID из Telegram-бота `@sirin_crp_bot` или откройте deep-link ссылку).
3. Нажмите центральную кнопку Сирин — активируется режим **Zapret 2 + Lazy VPN (`Z2`)**.

### 💻 Windows Desktop
1. Скачайте актуальный инсталлятор `Setup.exe` со страницы [релизов проекта на GitHub](https://github.com/goodbrat-lab/sirin-vpn/releases) и выполните установку.
2. В окне клиента укажите ваш ключ и нажмите **«Подключить»** (доступно встроенное автообновление прямо из настроек приложения).

### 🌐 Router (OpenWrt 21.x – 24.x `opkg` и 25.x `apk`)
Выполните одну команду в терминале SSH роутера (автоматически определит архитектуру `mips`, `mipsle`, `armv7`, `arm64`, `x86_64`, установит демон и веб-интерфейс LuCI):
```bash
wget -O - https://raw.githubusercontent.com/goodbrat-lab/sirin-vpn/master/apps/router/install.sh | sh
```
После установки откройте веб-интерфейс роутера: **Службы → Sirin VPN**, введите ключ и сохраните параметры. Обновлять клиент в дальнейшем можно одной кнопкой **«Проверить обновления»** прямо в LuCI.

---

## 📸 Скриншоты интерфейса

<div align="center">
  <h3>Мобильное приложение (Android — Cyber-Neon UI)</h3>
  <img src="docs/assets/screenshots/android-ui-main.png" alt="Sirin Android Disconnected" width="270" />
  &nbsp;&nbsp;&nbsp;&nbsp;
  <img src="docs/assets/screenshots/android-ui-connected.png" alt="Sirin Android Connected" width="270" />
</div>

<br/>

<div align="center">
  <h3>Клиент для Windows Desktop (Wintun + Zapret 2 + Lazy VPN)</h3>
  <img src="docs/assets/screenshots/desktop-ui-windows.png" alt="Sirin Windows Desktop UI" width="760" />
</div>

<br/>

<div align="center">
  <h3>Веб-интерфейс роутера (LuCI OpenWrt)</h3>
  <img src="docs/assets/screenshots/router-ui-luci.png" alt="Sirin OpenWrt LuCI UI" width="760" />
</div>

---

## 📚 Документация

Полная техническая документация и руководства пользователей структурированы в каталоге [**`docs/`**](docs/README.md) и на [**GitHub Pages**](https://goodbrat-lab.github.io/sirin-vpn/):

- **Установка и настройка:**
  - [Полное руководство по Router OpenWrt и LuCI UI](docs/ROUTER.md)
  - [Сборка и установка пакетов `.ipk` и `.apk` для OpenWrt](docs/OPENWRT_PACKAGES.md)
  - [Чеклист верификации установки на роутер](docs/ROUTER_INSTALL_VERIFICATION.md)
- **Управление и Telegram-бот:**
  - [Справочник команд и функций Telegram-бота](docs/BOT_COMMANDS.md)
  - [Карта интерактивных диалогов бота (UX Map)](docs/BOT_UX_MAP.md)
- **Диагностика и архитектура:**
  - [Известные проблемы и решения (Known Issues)](docs/KNOWN_ISSUES.md)
  - [План развития проекта (Roadmap)](docs/ROADMAP.md)
  - [Архитектура защиты «Невидимая вуаль»](docs/ROADMAP_INVISIBLE_VEIL.md)

---

## 🗺️ Roadmap проекта

Актуальные задачи и статус разработки отслеживаются в [docs/ROADMAP.md](docs/ROADMAP.md) и [docs/ROADMAP_INVISIBLE_VEIL.md](docs/ROADMAP_INVISIBLE_VEIL.md):

| Статус | Задача | Платформа / Компонент |
|:---:|---|---|
| 🟢 **Готово** | Нативный движок **Zapret 2** (стратегии десинхронизации DPI) + **Lazy On-Demand VPS VPN** (0 Байт в покое) | Android, Windows, Router |
| 🟢 **Готово** | Автоматический обход GeoIP-блокировок ИИ (ChatGPT, Claude, Gemini, Spotify, CAD) через принудительный VPN | Core Engine / Router |
| 🟢 **Готово** | Аварийный обход «Белых списков» через отечественные WebRTC TURN релеи (**VK Звонки / DION**) + **Dead Drop** (Яндекс Диск / S3) | Core Engine / All Clients |
| 🟢 **Готово** | Встроенное автообновление в один клик (In-App Updater) для OpenWrt LuCI и Windows Desktop | Router & Windows |
| 🟢 **Готово** | Раздельное туннелирование по подсетям РФ (8641 CIDR RIPE NCC) и портам (`Gaming Bypass`), автономные пакеты `.ipk` / `.apk` | Router & Windows |
| 🟡 **В работе** | Интеграция автоматического биллинга (Telegram Stars / СБП) в Telegram-боте | Server / Bot |
| 🔵 **В планах** | Клиент для iOS / iPadOS на базе NetworkExtension + Go Mobile | iOS Client |
| 🔵 **В планах** | Сборка Desktop-клиента под macOS и Linux | Desktop Client |

---

## ⚖️ Лицензия

Проект распространяется под открытой лицензией [MIT](LICENSE). Разработан на основе наработок Meridian VPN (JinComputers). Подробная информация об авторских правах приведена в [NOTICE](NOTICE).
