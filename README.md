<div align="center">
  <img src="docs/assets/logo.png" alt="Sirin VPN Logo" width="128" height="128" />
  <h1>Сирин (Sirin) VPN</h1>
  <p><strong>Быстрый, приватный и устойчивый к блокировкам ТСПУ VPN для всей экосистемы устройств</strong></p>

  <p>
    <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/releases/tag/v1.3.1"><img src="https://img.shields.io/badge/Android-v1.3.1-brightgreen.svg?logo=android&logoColor=white" alt="Android Release" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/releases/tag/router-v1.2.0"><img src="https://img.shields.io/badge/OpenWrt%20Router-v1.2.0-blueviolet.svg?logo=openwrt&logoColor=white" alt="Router Release" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/releases/tag/desktop-v1.0.0"><img src="https://img.shields.io/badge/Windows-v1.0.0-0078D6.svg?logo=windows&logoColor=white" alt="Windows Release" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/discussions"><img src="https://img.shields.io/badge/GitHub-Discussions-purple.svg?logo=github" alt="Discussions" /></a>
    <a href="https://goodbrat-lab.github.io/sirin-vpn/"><img src="https://img.shields.io/badge/Docs-GitHub%20Pages-informational.svg?logo=githubpages" alt="GitHub Pages" /></a>
  </p>
</div>

---

## ⚡ Что это

**Сирин (Sirin) VPN** — независимый клиент-серверный VPN-комплекс нового поколения, спроектированный специально для стабильной работы в условиях жестких сетевых фильтров и блокировок ТСПУ. Комплекс объединяет двухтранспортный протокол с автоматическим переключением (скоростной UDP Noise NK + маскированный под веб-трафик VLESS Reality XTLS-Vision), настраиваемое раздельное туннелирование (Split Tunneling с готовой базой Рунета) и собственную инфраструктуру авторизации без централизованных сторонних сервисов.

---

## 📱 Выберите платформу

| Платформа | Версия | Где скачать | Руководство по установке | Статус |
|:---|:---:|:---|:---|:---:|
| **Android** | `1.3.1` | [📦 Скачать APK (22.6 МБ)](https://github.com/goodbrat-lab/sirin-vpn/releases/download/v1.3.1/sirin-v1.3.1.apk) | [📖 Инструкция для Android](docs/README.md#11-android) | ✅ Стабильный |
| **Windows Desktop** | `1.0.0` | [📦 Скачать Setup.exe (7.1 МБ)](https://github.com/goodbrat-lab/sirin-vpn/releases/download/desktop-v1.0.0/Sirin-VPN-1.0.0-Setup.exe) | [📖 Инструкция для Windows](docs/README.md#2-windows-desktop) | ✅ Стабильный |
| **Router (OpenWrt)** | `1.2.0` | [📦 Релиз архивы (5 архитектур)](https://github.com/goodbrat-lab/sirin-vpn/releases/tag/router-v1.2.0) | [📖 Руководство OpenWrt & LuCI](docs/ROUTER.md) | ✅ Стабильный |
| **iOS / iPadOS** | — | *В разработке (архитектура NetworkExtension)* | [🔬 Исследование (Spike iOS)](docs/SPIKE_IOS.md) | ⏳ Скоро (Q1 2027) |
| **macOS / Linux** | — | *Запланировано на базе Wails & core/engine* | [🗺️ Roadmap](docs/ROADMAP.md) | ⏳ Скоро |

---

## 🚀 Быстрый старт

### 🤖 Android
1. Загрузите и установите [sirin-v1.3.1.apk](https://github.com/goodbrat-lab/sirin-vpn/releases/download/v1.3.1/sirin-v1.3.1.apk).
2. Запустите приложение и вставьте персональный ключ (UUID из Telegram-бота `@sirin_crp_bot` или откройте deep-link ссылку).
3. Нажмите кнопку питания в центре экрана — туннель поднимется автоматически.

### 💻 Windows Desktop
1. Скачайте инсталлятор [Sirin-VPN-1.0.0-Setup.exe](https://github.com/goodbrat-lab/sirin-vpn/releases/download/desktop-v1.0.0/Sirin-VPN-1.0.0-Setup.exe) и завершите установку.
2. В окне клиента укажите ваш ключ и нажмите **«Подключить»**.
3. Трафик системы защищён; статус отображается в системном трее.

### 🌐 Router (OpenWrt 24.x / 25.x)
Выполните одну команду в терминале SSH роутера (автоматически определит архитектуру `mips`, `mipsle`, `armv7`, `arm64`, `x86_64` и установит веб-интерфейс LuCI):
```bash
wget -O - https://raw.githubusercontent.com/goodbrat-lab/sirin-vpn/master/apps/router/install.sh | sh
```
После установки откройте веб-интерфейс роутера: **Службы → Sirin VPN**, введите ключ и сохраните параметры.

---

## 📸 Скриншоты

<div align="center">
  <h3>Мобильное приложение (Android)</h3>
  <img src="docs/assets/screenshots/android-ui-main.png" alt="Sirin Android Disconnected" width="280" />
  &nbsp;&nbsp;&nbsp;&nbsp;
  <img src="docs/assets/screenshots/android-ui-connected.png" alt="Sirin Android Connected" width="280" />
</div>

<br/>

<div align="center">
  <h3>Веб-интерфейс роутера (LuCI OpenWrt)</h3>
  <img src="docs/assets/screenshots/router-ui-luci.png" alt="Sirin OpenWrt LuCI UI" width="700" />
</div>

---

## 📚 Документация

Полная техническая документация и руководства пользователей структурированы в каталоге [**`docs/`**](docs/README.md) и на [**GitHub Pages**](https://goodbrat-lab.github.io/sirin-vpn/):

- **Установка и начало работы:**
  - [Полное руководство по Router OpenWrt и LuCI UI](docs/ROUTER.md)
  - [Чеклист верификации установки на роутер](docs/ROUTER_INSTALL_VERIFICATION.md)
- **Управление и Telegram-бот:**
  - [Справочник команд и функций Telegram-бота](docs/BOT_COMMANDS.md)
  - [Карта интерактивных диалогов бота (UX Map)](docs/BOT_UX_MAP.md)
- **Диагностика и поддержка:**
  - [Известные проблемы и решения (Known Issues)](docs/KNOWN_ISSUES.md)
  - [Как правильно оформить обращение (How To Report)](docs/HOW_TO_REPORT.md)
  - [Руководство по обратной связи](docs/FEEDBACK.md)

---

## 💬 Обратная связь и сообщество

- **Вопросы и обсуждения:** заходите в [GitHub Discussions](https://github.com/goodbrat-lab/sirin-vpn/discussions) — категории Q&A, идеи и отзывы.
- **Сообщить об ошибке:** воспользуйтесь нашими интерактивными формами баг-репортов для [Android](https://github.com/goodbrat-lab/sirin-vpn/issues/new?template=android_bug.yml), [Router](https://github.com/goodbrat-lab/sirin-vpn/issues/new?template=router_bug.yml) или [Windows](https://github.com/goodbrat-lab/sirin-vpn/issues/new?template=windows_bug.yml).
- **Задать вопрос:** [форма вопроса](https://github.com/goodbrat-lab/sirin-vpn/issues/new?template=question.yml).
- **Предложить улучшение:** [Feature Request](https://github.com/goodbrat-lab/sirin-vpn/issues/new?template=feature_request.yml).
- **Telegram-бот поддержки и ключей:** `@sirin_crp_bot`.

---

## 🗺️ Roadmap проекта

Актуальные задачи и статус разработки отслеживаются на странице [GitHub Projects](https://github.com/goodbrat-lab/sirin-vpn/projects) и в [docs/ROADMAP.md](docs/ROADMAP.md):

| Статус | Задача | Платформа / Компонент |
|:---:|---|---|
| 🟢 **Готово** | Настраиваемый Split Tunneling (Bypass/Only) + LuCI веб-интерфейс | Router (`v1.2.0`) |
| 🟢 **Готово** | Обновление визуального стиля «Крипто-защита» (Cyber-Neon UI) | Android (`v1.3.0`) |
| 🟢 **Готово** | Ротация пула доменов SNI (VK CDN, Кинопоиск, WB, IKEA, Microsoft) | Core Engine / Server |
| 🟡 **В работе** | Доработка клиентской стороны Telegram-бота (автопополнение и запросы триала) | Server / Bot |
| 🔵 **В планах** | Клиент для iOS / iPadOS на базе NetworkExtension | iOS Client |
| 🔵 **В планах** | Портирование Desktop-клиента под macOS и Linux | Desktop Client |
| 🔵 **В планах** | Поддержка Split Tunneling по портам (например, 80/443 через VPN) | Router Client |

---

## ⚖️ Лицензия

Проект распространяется под открытой лицензией [MIT](LICENSE). Разработан на основе наработок Meridian VPN (JinComputers). Подробная информация об авторских правах приведена в [NOTICE](NOTICE).
