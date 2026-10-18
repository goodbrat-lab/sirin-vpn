# Руководство по использованию Sirin Router (OpenWrt)

Клиент **Sirin VPN** для роутеров под управлением OpenWrt обеспечивает прозрачное туннелирование трафика всех устройств домашней сети через защищённый протокол (UDP Noise NK с автоматическим переключением на VLESS Reality при блокировках).

---

## 1. Системные требования

- **Операционная система:** OpenWrt 24.x (пакетный менеджер `opkg`) или OpenWrt 25.x (пакетный менеджер `apk`).
- **Свободная память:** не менее 10–15 МБ во Flash-памяти роутера (размер бинарного файла ~9–10 МБ).
- **Ядро:** поддержка TUN-устройств (`kmod-tun`).
- **Файрвол:** `nftables` (стандарт в OpenWrt 22+) либо классический `iptables`.
- **Сертификаты TLS:** установленные корневые сертификаты для загрузки по HTTPS.

---

## 2. Подключение по SSH

Подключитесь к командной строке вашего роутера с локального компьютера:

```bash
ssh root@192.168.1.1
```
*(Если IP-адрес роутера был изменён, укажите ваш актуальный адрес шлюза).*

---

## 3. Установка

### 3.1. Подготовка сертификатов (для чистых систем)

Если на роутере ещё не установлены пакеты поддержки SSL/TLS для `wget`, выполните предварительную команду:

```bash
# Для OpenWrt 24.x (opkg):
opkg update && opkg install ca-bundle wget-ssl

# Для OpenWrt 25.x (apk):
apk update && apk add ca-certificates wget-ssl
```

### 3.2. Автоматическая установка в одну команду

Выполните команду автоустановщика:

```bash
wget -O - https://raw.githubusercontent.com/goodbrat-lab/sirin-vpn/master/apps/router/install.sh | sh
```

Скрипт автоматически:
1. Определит архитектуру процессора роутера (`mips`, `mipsle`, `armv7`, `arm64` или `x86_64`).
2. Загрузит соответствующий релизный архив с GitHub.
3. Разместит бинарник `/usr/bin/sirin-client` и init-скрипт `/etc/init.d/sirin`.
4. Создаст шаблон конфигурации `/etc/sirin/client.toml` (если файл ещё не существует).
5. Установит необходимые зависимости (`kmod-tun`, `nftables`, `ip-full`).
6. Включит службу в автозагрузку системы.

### 3.3. Ручная установка (без прямого выхода в интернет)

Если роутер временно не имеет доступа в сеть:
1. Загрузите tar.gz архив нужной архитектуры со страницы [GitHub Releases](https://github.com/goodbrat-lab/sirin-vpn/releases).
2. Скопируйте архив на роутер:
   ```bash
   scp sirin-client-1.0.1-<arch>.tar.gz root@192.168.1.1:/tmp/
   ```
3. Распакуйте архив и расставьте файлы:
   ```bash
   tar -xzf /tmp/sirin-client-1.0.1-<arch>.tar.gz -C /tmp/
   cp /tmp/sirin-client-<arch> /usr/bin/sirin-client
   chmod +x /usr/bin/sirin-client
   mkdir -p /etc/sirin
   cp /tmp/client.toml.example /etc/sirin/client.toml
   cp /tmp/init.d/sirin /etc/init.d/sirin
   chmod +x /etc/init.d/sirin
   /etc/init.d/sirin enable
   ```

---

## 4. Настройка (`/etc/sirin/client.toml`)

Откройте файл конфигурации для редактирования:

```bash
vi /etc/sirin/client.toml
```

### Пример конфигурационного файла

```toml
# Sirin Client для OpenWrt
gateway = "crp.guseynovia.ru"
password = "REPLACE_WITH_YOUR_UUID"  # получить в @sirin_crp_bot
device_id = "router-01"

tun_name = "sirin0"
tun_mtu = 1280

lan_interface = "br-lan"
wan_interface = "wan"

api_url = "https://crp.guseynovia.ru"
ru_direct = false

# Fallback: если API недоступен, использовать статический пул узлов
[[ladder]]
name = "A"
transport = "udp"
host = "94.183.159.114"
port = 56005

[[ladder]]
name = "B"
transport = "vless"
host = "94.183.159.114"
port = 443
sni = "sun8-21.userapi.com"
pbk = "4ll5_ouUJr8P8CCxasWNcNu0Lb7tSIDacGmRskweoH4"
sid = "8283f99ec6a0f52e"
```

### Описание параметров

| Параметр | Тип | По умолчанию | Описание |
|---|---|---|---|
| `gateway` | строка | `crp.guseynovia.ru` | Домен или IP шлюза управления VPN |
| `password` | строка | *(обязательно)* | Ваш персональный UUID (выдаётся в Telegram-боте `@sirin_crp_bot`) |
| `device_id` | строка | `router-01` | Идентификатор устройства для отображения в сессиях |
| `tun_name` | строка | `sirin0` | Имя создаваемого виртуального сетевого интерфейса |
| `tun_mtu` | число | `1280` | MTU виртуального интерфейса (оптимально для обхода фрагментации) |
| `lan_interface` | строка | `br-lan` | Интерфейс локальной сети (мост LAN-портов роутера) |
| `wan_interface` | строка | `""` (авто) | Внешний интерфейс провайдера (если не указан, определяется автоматически) |
| `wan_gateway` | строка | `""` (авто) | Шлюз провайдера (если не указан, определяется по маршруту по умолчанию) |
| `api_url` | строка | `https://crp.guseynovia.ru` | URL API для динамического получения рабочих параметров лестницы транспортов |
| `ru_direct` | булево | `false` | Split tunneling: направлять трафик к 4945 диапазонам IP РФ напрямую через WAN в обход VPN |
| `[[ladder]]` | список | — | Резервный статический список транспортов при недоступности API (`udp`, `vless`) |

> [!TIP]
> Если вам требуется, чтобы российские сервисы (банки, Госуслуги, Яндекс) открывались на максимальной скорости вашего провайдера без VPN, установите:
> ```toml
> ru_direct = true
> ```

---

## 5. Запуск и проверка

### 5.1. Запуск службы

```bash
/etc/init.d/sirin start
```

### 5.2. Мониторинг журнала событий

```bash
logread -f | grep sirin
```

Фактический вывод при успешном подключении:

```text
Запуск Sirin VPN Client v1.0.1...
[net] Обнаружен WAN: iface=eth0, gw=192.168.1.1
[tun] Создан интерфейс sirin0 (MTU=1280)
[ladder] Запрос конфигурации с API: https://crp.guseynovia.ru...
[ladder] Подключение к шлюзу: crp.guseynovia.ru (хостов для исключения: 1)
[engine] Туннель готов: connection established (transport: udp)
[route] Настройка интерфейса sirin0: IP=10.77.77.10/24, MTU=1280
[route] RuDirect: 4945 CIDR добавлено напрямую через WAN
[route] Маршрутизация и NAT успешно активированы
```

### 5.3. Проверка внешнего IP-адреса

1. **Проверка туннелирования внешнего трафика (должен отображаться IP вашего VPN-сервера):**
   ```bash
   wget -qO- https://api.ipify.org; echo
   ```
   *(Альтернатива: `wget -qO- https://ifconfig.me/ip; echo`)*.

2. **Проверка раздельного маршрутирования (Split Tunneling при `ru_direct = true`):**
   Каноническая проверка через таблицу маршрутизации ядра роутера (маршрут к DNS Яндекса `77.88.8.8` должен идти через шлюз провайдера, а не через `sirin0`):
   ```bash
   ip route get 77.88.8.8 | grep -q sirin0 && echo "ЧЕРЕЗ VPN (ошибка)" || echo "НАПРЯМУЮ (ок)"
   ```

---

## 6. Веб-интерфейс (LuCI)

Начиная с версии `router-v1.1.0`, для клиента доступно веб-приложение `luci-app-sirin` для стандартного интерфейса OpenWrt (LuCI).

### 6.1. Архитектура веб-интерфейса

- **Технология UI:** LuCI JavaScript (совместимо с OpenWrt 24.x/25.x, работает в современных темах без устаревшего Lua CBI).
- **Бэкенд:** `rpcd` exec-плагин `/usr/libexec/rpcd/sirin` (регистрирует объект ubus `sirin` с разграничением прав через ACL).
- **Источник истины:** конфигурационный файл `/etc/sirin/client.toml`. Веб-интерфейс управляет только верхнеуровневыми параметрами: `gateway`, `password`, `device_id`, `ru_direct`, `tun_mtu`. Массив `[[ladder]]` и дополнительные настройки сохраняются без изменений.
- **Безопасность:** перед каждым изменением создаётся автоматическая резервная копия `/etc/sirin/client.toml.bak.<timestamp>`.
- **Автоопределение:** интерфейс устанавливается автоматически, если LuCI обнаружен в системе (`[ -d /www/luci-static/resources ] && [ -d /usr/share/luci ]`). При необходимости установку интерфейса можно форсировать флагом `--with-ui` или отключить флагом `--no-ui`.

### 6.2. Доступ к интерфейсу

После установки откройте веб-интерфейс роутера в браузере:
- **URL:** `http://192.168.1.1/cgi-bin/luci/admin/services/sirin` (меню **Службы** → **Sirin VPN**).
- **Авторизация:** стандартная учётная запись администратора роутера (`root`).

### 6.3. Состав страницы управления

Интерфейс разделен на три логических блока:

1. **«Состояние»:**
   - Индикатор статуса («Работает» / «Остановлен»), текущий PID, версия клиента и время непрерывной работы (uptime).
   - Текущий активный транспорт туннеля (UDP или VLESS Reality).
   - Кнопки оперативного управления: **Запустить**, **Остановить**, **Перезапустить**.
   - Переключатель автозапуска службы при старте роутера.
   - Автоматическое обновление данных каждые 5 секунд.

2. **«Настройки»:**
   - Поля ввода: `Шлюз (Gateway)`, `Ключ доступа (UUID)`, `ID устройства`, `MTU (576–1500)` и опция `RuDirect` (прямой доступ к сервисам РФ).
   - Поле пароля маскирует сохранённый ключ и имеет кнопку показа/скрытия.
   - Кнопки: **Сохранить** (с валидацией и предложением перезапустить туннель) и **Отмена** (сброс к текущим значениям).

3. **«Диагностика»:**
   - **Проверить подключение:** определяет внешний публичный IP и проверяет корректность прямого маршрута для Рунета.
   - **Показать логи:** просмотр последних записей журнала работы клиента в удобном модальном окне с возможностью обновления.

### 6.4. Установка LuCI на чистых системах

Если LuCI не был установлен в вашей прошивке OpenWrt:
```bash
# Для OpenWrt 24.x (opkg):
opkg update && opkg install luci

# Для OpenWrt 25.x (apk):
apk update && apk add luci

# Затем повторите установку Sirin с флагом --with-ui:
wget -O - https://raw.githubusercontent.com/goodbrat-lab/sirin-vpn/master/apps/router/install.sh | sh -s -- --with-ui
```

---

## 7. Управление службой (CLI)

| Действие | Команда |
|---|---|
| Запустить VPN | `/etc/init.d/sirin start` |
| Остановить VPN | `/etc/init.d/sirin stop` |
| Перезапустить | `/etc/init.d/sirin restart` |
| Включить автозапуск | `/etc/init.d/sirin enable` |
| Отключить автозапуск | `/etc/init.d/sirin disable` |
| Проверить статус процесса | `ps | grep [s]irin-client` |
| Проверить статус в procd | `ubus call service list '{"name":"sirin"}'` |

---

## 8. Обновление

Процесс обновления полностью идемпотентен: скрипт `install.sh` **никогда не перезаписывает** существующий `/etc/sirin/client.toml`. Ваш UUID и персональные настройки сохраняются.

Для обновления на свежую версию выполните:

```bash
/etc/init.d/sirin stop
wget -O - https://raw.githubusercontent.com/goodbrat-lab/sirin-vpn/master/apps/router/install.sh | sh
/etc/init.d/sirin start
```

---

## 9. Полное удаление

При штатной остановке через `/etc/init.d/sirin stop` клиент автоматически удаляет маршруты `0.0.0.0/1`, `128.0.0.0/1`, диапазоны RuDirect и таблицу трансляции адресов в файрволе.

Для полного удаления всех файлов программы выполните:

```bash
/etc/init.d/sirin stop
/etc/init.d/sirin disable
rm -f /etc/init.d/sirin /usr/bin/sirin-client
rm -rf /etc/sirin
ip link del sirin0 2>/dev/null || true
```

### Экстренная ручная очистка маршрутов

Если процесс был принудительно завершён без вызова `stop` (например, `kill -9`), и на роутере временно отсутствует доступ в сеть:

```bash
ip route del 0.0.0.0/1 2>/dev/null || true
ip route del 128.0.0.0/1 2>/dev/null || true
nft delete table ip sirin 2>/dev/null || true
iptables -t nat -D POSTROUTING -o sirin0 -j MASQUERADE 2>/dev/null || true
```

---

## 10. Диагностика проблем

1. **`wget: certificate verification failed` или `bad address`:**
   В системе отсутствуют актуальные SSL-сертификаты. Выполните:
   - для OpenWrt 24.x: `opkg update && opkg install ca-bundle wget-ssl`
   - для OpenWrt 25.x: `apk update && apk add ca-certificates wget-ssl`
2. **`Ключ доступа (password / UUID) не указан в /etc/sirin/client.toml`:**
   Откройте `/etc/sirin/client.toml` и замените плейсхолдер `REPLACE_WITH_YOUR_UUID` на реальный UUID из Telegram-бота `@sirin_crp_bot`.
3. **`WARNING: /dev/net/tun не найден`:**
   Убедитесь, что установлен модуль ядра TUN:
   ```bash
   opkg install kmod-tun   # или apk add kmod-tun
   lsmod | grep tun
   ls -la /dev/net/tun
   ```
4. **`logread: command not found`:**
   На некоторых минималистичных прошивках системный журнал выводится через `dmesg` или в `/var/log/messages`. Вы также можете запустить бинарник вручную для мгновенной проверки логов:
   ```bash
   /usr/bin/sirin-client -config /etc/sirin/client.toml
   ```
5. **Пропал интернет после непредвиденной перезагрузки клиента:**
   Выполните команды из подраздела [Экстренная ручная очистка маршрутов](#экстренная-ручная-очистка-маршрутов).

---

## 11. Поддерживаемые архитектуры

Список поддерживаемых архитектур процессоров и типовых устройств приведён в основном файле проекта: [README.md](../README.md#архитектуры).
