# Программа приёмочных испытаний Router v1.3.0 (XHTTP + Reality)

Документ содержит регламент проверки и валидации роутерного клиента Sirin на базе OpenWrt при внедрении протокола **XHTTP+Reality (P1)** и обратной совместимости с существующими транспортами (VLESS-Reality :443, UDP :56005).

---

## 1. Подготовка и обновление роутера до v1.3.0

1. Скачайте релизный архив для архитектуры вашего роутера (например, `sirin-client-1.3.0-mipsle.tar.gz` или `armv7` / `arm64`):
   ```sh
   wget https://github.com/goodbrat-lab/sirin-vpn/releases/download/router-v1.3.0/sirin-client-1.3.0-<arch>.tar.gz
   tar -xzf sirin-client-1.3.0-<arch>.tar.gz
   cd sirin-client-1.3.0-<arch>
   sh install.sh --with-ui
   ```
2. Проверьте установленную версию:
   ```sh
   /usr/bin/sirin-client --version
   # Ожидаемый вывод:
   # sirin-client v1.3.0 (OpenWrt/Linux router client)
   ```

---

## 2. Чек-лист испытаний

### Тест 1: Проверка XHTTP+Reality (:8443)
1. В `/etc/sirin/client.toml` настройте транспорт `xhttp` в секции `[[ladder]]` (или убедитесь, что API `/v1/params` отдаёт кандидат xhttp):
   ```toml
   [[ladder]]
   name = "XHTTP"
   transport = "xhttp"
   host = "crp.guseynovia.ru"
   port = 8443
   path = "/s1r1nxhttp2026k"
   mode = "stream-one"
   sni = "sun8-21.userapi.com"
   pbk = "4ll5_ouUJr8P8CCxasWNcNu0Lb7tSIDacGmRskweoH4"
   sid = "8283f99ec6a0f52e"
   ```
2. Перезапустите службу:
   ```sh
   /etc/init.d/sirin restart
   logread -f | grep sirin
   ```
   **Ожидаемый вывод:**
   ```text
   [ladder] Подключение через xhttp...
   [engine] Туннель готов (transport: xhttp)
   ```
3. Проверка выхода в интернет и подмены IP:
   ```sh
   curl -4 https://api.ipify.org
   ```
   **Критерий успеха:** Возвращается публичный IP шлюза Sirin (`<ip_сервера>`), а не локальный IP интернет-провайдера.

4. Замер скорости (speedtest):
   - Замерить ping, download, upload через speedtest CLI или с клиентского устройства за роутером.
   - Зафиксировать значения: `Ping: ... ms | DL: ... Mbps | UL: ... Mbps`.

---

### Тест 2: Проверка регрессии VLESS-Reality (:443)
1. В `/etc/sirin/client.toml` отключите XHTTP и оставьте только транспорт `vless` (:443).
2. Перезапустите службу:
   ```sh
   /etc/init.d/sirin restart
   ```
3. Проверка IP:
   ```sh
   curl -4 https://api.ipify.org
   ```
   **Критерий успеха:** Возвращается IP сервера (`<ip_сервера>`).
4. Замер скорости VLESS:
   - Сравнить с замером XHTTP. Допустимое отклонение скорости XHTTP vs VLESS обусловлено накладными расходами обрамления HTTP/2 stream-one.

---

### Тест 3: Проверка регрессии UDP Direct (:56005)
1. В `/etc/sirin/client.toml` настройте транспорт `udp` (:56005).
2. Перезапустите службу:
   ```sh
   /etc/init.d/sirin restart
   ```
3. Проверка IP:
   ```sh
   curl -4 https://api.ipify.org
   ```
   **Критерий успеха:** Возвращается IP сервера (`<ip_сервера>`).

---

## 3. Таблица результатов приёмочных испытаний

| Тест / Транспорт | Параметры | Ожидаемый IP | Фактический IP | Замер скорости (DL / UL / Ping) | Статус |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **XHTTP+Reality** | `:8443`, path `/s1r1nxhttp2026k`, stream-one | `<ip_сервера>` | `<ip_сервера>` | TCP :8443 ESTABLISHED + ip.me = сервер | ✅ (v1.3.1) |
| **VLESS+Reality** | `:443`, Reality-TCP | `<ip_сервера>` | `<ip_сервера>` | *Заполняется владельцем* | ✅ / ⏳ |
| **UDP Direct** | `:56005`, Sirin OBFS | `<ip_сервера>` | `<ip_сервера>` | *Заполняется владельцем* | ✅ / ⏳ |
