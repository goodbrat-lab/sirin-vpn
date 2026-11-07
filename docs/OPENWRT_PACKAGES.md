# Сборка и установка пакетов OpenWrt (.ipk и .apk)

Проект **Sirin VPN** поддерживает два формата пакетов для роутеров под управлением OpenWrt:
1. **`.ipk`** — для пакетного менеджера `opkg` (OpenWrt версий 21.x, 22.x, 23.x, 24.x).
2. **`.apk`** — для пакетного менеджера `apk` (OpenWrt версий 25.x и Snapshot).

---

## 📦 Доступные пакеты

В каталоге `apps/router/dist/` генерируются пакеты:

| Пакет | Формат | Архитектура | Описание |
| :--- | :--- | :--- | :--- |
| `sirin-full_<ver>_<arch>.ipk` | `.ipk` | `mips`, `mipsle`, `armv7`, `arm64`, `x86_64` | **All-in-one бандл**: демон + LuCI Web UI. Устанавливается одной командой. |
| `sirin-full-<ver>-<arch>.apk` | `.apk` | `mips`, `mipsle`, `armv7`, `arm64`, `x86_64` | All-in-one бандл для OpenWrt 25.x (Alpine apk). |
| `sirin-client_<ver>_<arch>.ipk` | `.ipk` | `mips`, `mipsle`, `armv7`, `arm64`, `x86_64` | Демон `sirin-client` и init-скрипт procd (без веб-интерфейса). |
| `sirin-client-<ver>-<arch>.apk` | `.apk` | `mips`, `mipsle`, `armv7`, `arm64`, `x86_64` | Демон `sirin-client` для OpenWrt 25.x. |
| `luci-app-sirin_<ver>_all.ipk` | `.ipk` | `all` (универсальный) | Веб-интерфейс LuCI, меню и RPC-интеграция для `opkg`. |
| `luci-app-sirin-<ver>.apk` | `.apk` | `noarch` (универсальный) | Веб-интерфейс LuCI для OpenWrt 25.x. |

---

## 🚀 Установка на роутер

### 1. OpenWrt 24.x и ниже (через `opkg`)

Скопируйте пакет на роутер через `scp` или скачайте по прямой ссылке:

```sh
# 1. Обновление списков и установка системных зависимостей
opkg update
opkg install kmod-tun ca-bundle nftables ip-full

# 2. Установка готового бандла (на примере популярного MIPSLE)
opkg install /tmp/sirin-full_1.4.0-1_mipsle.ipk

# Либо раздельная установка демона и LuCI:
opkg install /tmp/sirin-client_1.4.0-1_mipsle.ipk
opkg install /tmp/luci-app-sirin_1.4.0-1_all.ipk
```

После установки:
- Веб-интерфейс доступен в LuCI: **Сеть (Network) / Службы (Services) → Sirin VPN**.
- Служба автоматически зарегистрирована в `procd`: `/etc/init.d/sirin start`.

---

### 2. OpenWrt 25.x / Snapshot (через `apk`)

OpenWrt 25.x использует пакетный менеджер `apk` из Alpine Linux:

```sh
# 1. Установка системных зависимостей
apk update
apk add kmod-tun ca-certificates nftables

# 2. Установка пакета (--allow-untrusted для локальных неподписанных пакетов)
apk add --allow-untrusted /tmp/sirin-full-1.4.0-r1-mipsle.apk

# Либо раздельно:
apk add --allow-untrusted /tmp/sirin-client-1.4.0-r1-mipsle.apk
apk add --allow-untrusted /tmp/luci-app-sirin-1.4.0-r1.apk
```

---

## 🛠 Автономная сборка пакетов

Для сборки всех 22 пакетов (`.ipk`, `.apk`, `.tar.gz`) не требуется 15 ГБ OpenWrt SDK. Встроенный автономный сборщик написан на чистом Go (`apps/router/packager/main.go`):

### В Windows (PowerShell):
```powershell
cd apps/router
.\build-all.ps1
```

### В Linux / macOS (Bash):
```bash
cd apps/router
./build-all.sh
```

Сборщик компилирует бинарники под все 5 архитектур и упаковывает валидные архивы со строгими POSIX-правами (0755 для исполняемых файлов, 0644 для конфигов, `uid/gid = 0 (root)`).

---

## 🏗 Сборка через официальный OpenWrt SDK

Для интеграции в кастомные прошивки или сборки через официальный OpenWrt SDK подготовлены Makefiles:
- [`openwrt/sirin-client/Makefile`](file:///c:/Antigravity_project/Fork%20Meridian/meridian-fork/apps/router/openwrt/sirin-client/Makefile)
- [`openwrt/luci-app-sirin/Makefile`](file:///c:/Antigravity_project/Fork%20Meridian/meridian-fork/apps/router/openwrt/luci-app-sirin/Makefile)

### Инструкция для SDK:
1. Скопируйте папки пакетов в дерево SDK:
   ```sh
   cp -r apps/router/openwrt/sirin-client package/
   cp -r apps/router/openwrt/luci-app-sirin package/
   ```
2. Выберите пакеты в `make menuconfig` (`Network -> VPN -> sirin-client` и `LuCI -> 3. Applications -> luci-app-sirin`).
3. Скомпилируйте:
   ```sh
   make package/sirin-client/compile V=s
   make package/luci-app-sirin/compile V=s
   ```
   Готовые пакеты появятся в `bin/packages/<target>/base/` и `bin/packages/<target>/luci/`.
