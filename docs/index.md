<div align="center">
  <img src="assets/logo.png" alt="Sirin VPN Logo" width="128" height="128" />
  <h1>Сирин (Sirin) VPN</h1>
  <p><strong>Быстрый, приватный и устойчивый к блокировкам ТСПУ VPN для всей экосистемы устройств</strong></p>

  <p>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/blob/master/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/releases/tag/v1.3.1"><img src="https://img.shields.io/badge/Android-v1.3.1-brightgreen.svg?logo=android&logoColor=white" alt="Android Release" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/releases/tag/router-v1.2.0"><img src="https://img.shields.io/badge/OpenWrt%20Router-v1.2.0-blueviolet.svg?logo=openwrt&logoColor=white" alt="Router Release" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/releases/tag/desktop-v1.0.0"><img src="https://img.shields.io/badge/Windows-v1.0.0-0078D6.svg?logo=windows&logoColor=white" alt="Windows Release" /></a>
    <a href="https://github.com/goodbrat-lab/sirin-vpn/discussions"><img src="https://img.shields.io/badge/GitHub-Discussions-purple.svg?logo=github" alt="Discussions" /></a>
  </p>
</div>

---

## ⚡ О проекте

**Сирин (Sirin) VPN** — независимый клиент-серверный VPN-комплекс, спроектированный специально для стабильной работы в условиях жестких сетевых фильтров и блокировок ТСПУ. Комплекс объединяет двухтранспортный протокол с автоматическим переключением (скоростной UDP Noise NK + маскированный под веб-трафик VLESS Reality XTLS-Vision), настраиваемое раздельное туннелирование (Split Tunneling с готовой базой Рунета) и собственную инфраструктуру авторизации без централизованных сторонних сервисов.

---

## 📱 Выберите платформу

| Платформа | Версия | Где скачать | Руководство | Статус |
|:---|:---:|:---|:---|:---:|
| **Android** | `1.3.1` | [📦 Скачать APK (22.6 МБ)](https://github.com/goodbrat-lab/sirin-vpn/releases/download/v1.3.1/sirin-v1.3.1.apk) | [📖 Инструкция для Android](README.md#11-android) | ✅ Стабильный |
| **Windows Desktop** | `1.0.0` | [📦 Скачать Setup.exe (7.1 МБ)](https://github.com/goodbrat-lab/sirin-vpn/releases/download/desktop-v1.0.0/Sirin-VPN-1.0.0-Setup.exe) | [📖 Инструкция для Windows](README.md#2-windows-desktop) | ✅ Стабильный |
| **Router (OpenWrt)** | `1.2.0` | [📦 Релиз архивы (5 архитектур)](https://github.com/goodbrat-lab/sirin-vpn/releases/tag/router-v1.2.0) | [📖 Руководство OpenWrt & LuCI](ROUTER.md) | ✅ Стабильный |
| **iOS / iPadOS** | — | *В разработке (архитектура NetworkExtension)* | [🔬 Исследование (Spike iOS)](SPIKE_IOS.md) | ⏳ Скоро (Q1 2027) |
| **macOS / Linux** | — | *Запланировано на базе Wails & core/engine* | [🗺️ Roadmap](ROADMAP.md) | ⏳ Скоро |

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

## 📸 Интерфейс клиентов

<div align="center">
  <h3>Мобильное приложение (Android)</h3>
  <img src="assets/screenshots/android-ui-main.png" alt="Sirin Android Disconnected" width="280" />
  &nbsp;&nbsp;&nbsp;&nbsp;
  <img src="assets/screenshots/android-ui-connected.png" alt="Sirin Android Connected" width="280" />
</div>

<br/>

<div align="center">
  <h3>Веб-интерфейс роутера (LuCI OpenWrt)</h3>
  <img src="assets/screenshots/router-ui-luci.png" alt="Sirin OpenWrt LuCI UI" width="700" />
</div>

---

## 📚 База знаний и документация

- [Индекс всей документации](README.md)
- [Руководство по OpenWrt роутерам и LuCI](ROUTER.md)
- [Чеклист верификации роутера](ROUTER_INSTALL_VERIFICATION.md)
- [Справочник команд Telegram-бота](BOT_COMMANDS.md)
- [Карта интерфейса бота (UX Map)](BOT_UX_MAP.md)
- [Известные проблемы и решения (Known Issues)](KNOWN_ISSUES.md)
- [Архитектурные решения (Decision Matrix)](DECISION_MATRIX.md)
- [План развития (Roadmap)](ROADMAP.md)

---

## 💬 Сообщество

- [GitHub Discussions](https://github.com/goodbrat-lab/sirin-vpn/discussions)
- [Репозиторий на GitHub](https://github.com/goodbrat-lab/sirin-vpn)
- [Сообщить об ошибке](https://github.com/goodbrat-lab/sirin-vpn/issues/new/choose)
