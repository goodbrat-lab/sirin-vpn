# Система многопутевого мониторинга и автоматического fallback (Watchdog & API)

Документ описывает архитектуру распределённого мониторинга задержек и потерь (RTT / packet loss) между узлами кластера Sirin VPN, эндпоинты Sirin API, пороги деградации и логику динамического переключения маршрутов без модификации клиентской базы (`clients.json`).

---

## 1. Архитектура мониторинга

Мониторинг построен по топологии полного треугольника между тремя географическими точками:
1. **Нидерланды (NL, `94.183.159.114`)** — основной узел выгрузки (VPN Egress & Sirin API).
2. **Москва (Relay 1, `185.182.65.219`)** — российский входной релей.
3. **Германия (Relay 2, `45.38.228.142`)** — европейский резервный релей.

### Схема телеметрии

```
   [ Москва: 185.182.65.219 ]
       │ ▲             ▲ │
       │ │ RTT ~52ms   │ │ RTT ~39ms
       │ │             │ │
       ▼ │             │ ▼
[ Германия: 45.38.228.142 ] ◄── RTT ~16ms ──► [ Нидерланды: 94.183.159.114 ]
                                                  (Sirin API :8080)
```

Каждая нода выполняет циклический probe раз в 60 секунд (`/usr/local/bin/watchdog_multi_path.sh` по cron):
- 10 ICMP-пакетов до каждого удалённого узла.
- Измерение среднего RTT (`rtt_ms`) и процента потерь (`loss_pct`).
- Контрольный HTTP GET до эндпоинта `/health`.
- Отправка телеметрии через `POST https://crp.guseynovia.ru/v1/admin/monitor`.

---

## 2. Пороги деградации узлов (Degraded Thresholds)

Узел помечается статусом **`degraded`**, если выполняется хотя бы одно из условий:
1. **Потери пакетов (`loss_pct`):** > **5%**.
2. **Задержка (`rtt_ms`):** > **150 мс**.
3. **Healthcheck:** HTTP-код ответа != `200` (если применимо).

В противном случае статус узла — **`ok`**.
Время жизни отчёта (TTL) в памяти API — **3 минуты**. Если отчёт устарел (например, нода недоступна), узел не блокирует fallback навсегда.

---

## 3. Эндпоинты Sirin API

### 3.1. Приём телеметрии: `POST /v1/admin/monitor`
- **Авторизация:** `Authorization: Bearer <ADMIN_TOKEN>`
- **Запрос:**
```json
{
  "node": "moscow",
  "rtt_ms": 38.6,
  "loss_pct": 0.0,
  "status": "ok"
}
```
- **Ответ (200 OK):**
```json
{
  "contract": "v1",
  "status": "ok",
  "report": {
    "node": "moscow",
    "rtt_ms": 38.6,
    "loss_pct": 0,
    "status": "ok",
    "updated_at": "2026-10-27T20:23:51.941859725Z"
  }
}
```

### 3.2. Просмотр состояния кластера: `GET /v1/admin/monitor`
- **Авторизация:** `Authorization: Bearer <ADMIN_TOKEN>`
- **Ответ (200 OK):**
```json
{
  "contract": "v1",
  "status": "answered",
  "reports": {
    "germany": {"node": "germany", "rtt_ms": 16.0, "loss_pct": 0, "status": "ok", "updated_at": "..."},
    "moscow": {"node": "moscow", "rtt_ms": 38.6, "loss_pct": 0, "status": "ok", "updated_at": "..."},
    "germany_from_moscow": {"node": "germany_from_moscow", "rtt_ms": 55.7, "loss_pct": 0, "status": "ok", "updated_at": "..."},
    "moscow_from_germany": {"node": "moscow_from_germany", "rtt_ms": 51.1, "loss_pct": 0, "status": "ok", "updated_at": "..."}
  }
}
```

---

## 4. Логика динамического fallback в `/v1/params`

Когда клиент опрашивает `/v1/params?key=<UUID>`:
1. API считывает назначенный клиенту `c.RelayID` из `clients.json`.
2. Если `c.RelayID == 1` (Москва) и узел `moscow` в состоянии `degraded`:
   - API логирует: `[WARN] client assigned relay_id=1 is DEGRADED! Finding healthy fallback`.
   - Если Direct NL здоров -> клиенту выдаётся Direct NL (`effectiveRelayID = 0`).
   - Если Direct NL также деградирован -> клиенту выдаётся Relay 2 (`effectiveRelayID = 2`).
   - В ответ добавляются HTTP-заголовки:
     * `X-Fallback-Active: true`
     * `X-Effective-Relay: 0`
3. Если `c.RelayID == 0` (Direct NL) и Direct NL деградирован:
   - Клиенту выдаётся Relay 2 (Германия, `effectiveRelayID = 2`), если он здоров.
   - Если Германия недоступна -> выдаётся Relay 1 (Москва).
4. **Важное требование безопасности:** база `clients.json` **НЕ перезаписывается**. Изменение маршрута действует только на лету в отдаваемом ответе `/v1/params`. Как только узел восстанавливается (`status: ok`), при следующем запросе параметров (TTL 300 с) клиент мгновенно возвращается на свой целевой маршрут.

---

## 5. Telegram-алертинг

При смене состояния узла `ok` -> `degraded`:
- Sirin API в асинхронном режиме отправляет уведомление администратору в Telegram:
  ```
  ⚠️ [Sirin Monitor] Узел moscow перешёл в состояние DEGRADED!
  RTT: 185.0 ms, Потери: 12.0%
  Автоматический fallback активирован в /v1/params.
  ```
- При восстановлении узла (`degraded` -> `ok`):
  ```
  ✅ [Sirin Monitor] Узел moscow восстановился (OK).
  RTT: 38.6 ms, Потери: 0.0%.
  ```
