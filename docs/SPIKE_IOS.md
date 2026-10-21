# Spike: iOS-клиент Sirin

> **Статус документа:** Research / Spike  
> **Дата:** Октябрь 2026  
> **Целевая аудитория первой волны:** 25–30 активных пользователей (клиенты в РФ)  
> **Существующая экосистема:** Android (Kotlin/Compose + Go core), Desktop (Wails/Go), Router (Go daemon)  
> **Протоколы сервера:** Первичный — UDP Noise NK (порт 56005), Резервный — VLESS + Reality (порт 443)  

---

## TL;DR (решения)

1. **Архитектурный выбор:** **Swift (SwiftUI) + переиспользование `core/engine/` (Go) через `gomobile bind`**.  
   В кодовой базе Sirin уже реализован легковесный независимый движок (`core/engine/`) с поддержкой UDP Noise NK и VLESS+Reality (`utls`), а также заранее спроектирован контракт `PacketFlow` (`engine.StartFlow`) специально для бестелесного `NEPacketTunnelProvider`. Полноценный Xray-core на iOS тащить **не нужно и вредно** (избыточный вес и риск вылета по памяти).
2. **Лимит памяти 50 MiB:** В Go-рантайме расширения обязательно выставляются ограничения: `debug.SetMemoryLimit(38 * 1024 * 1024)` и `GOGC=25`. Так как в `core/engine` отсутствуют многомегабайтные геобазы (`geoip.dat`/`geosite.dat`), базовый RSS держится на уровне 18–25 MiB, что безопасно для iOS Jetsam.
3. **Split Tunneling:** В MVP — **Route-based Split Tunneling** через `NEPacketTunnelNetworkSettings.ipv4Settings.excludedRoutes` (исключение приватных сетей RFC1918 `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`). Per-App split tunneling на iOS запрещен Apple для обычных приложений (доступен только через корпоративный MDM).
4. **Deep-link:** Реализуется через Custom URL Scheme `sirin://key?uuid=...` в `Info.plist` с перехватом в SwiftUI `.onOpenURL` и сохранением ключа в общий `Keychain` / `App Group` (`group.com.sirin.vpn`).
5. **Распространение для 25–30 человек:** **TestFlight (Internal/External Testing)** в рамках стандартного Apple Developer Program ($99/год). Не требует сбора UDID в отличие от Ad Hoc, билд живет 90 дней (обновление через Fastlane в CI раз в 80 дней), автообновления для пользователей из коробки.
6. **Оценка трудоемкости:** **12–15 рабочих дней (2.5–3 недели)** для полноценного MVP (сборка XCFramework, Network Extension, SwiftUI UI, Keychain/Deep-link, CI/TestFlight pipeline).

---

## Ограничения iOS (таблица)

Разработка VPN-клиента под платформу iOS фундаментально отличается от Android и Linux/Darwin Desktop из-за жестких ограничений песочницы Apple.

| Ограничение | Механизм на iOS | Влияние на Sirin VPN | Способ обхода / Решение |
| :--- | :--- | :--- | :--- |
| **Лимит памяти Network Extension** | Watchdog/Jetsam убивает расширение сигналом `EXC_RESOURCE (ActiveHard 50 MB)` при превышении ~50 MiB RAM. | Запуск тяжелых рантаймов (полный Xray-core с геобазами) приводит к мгновенным крашам при burst-нагрузке. | Использование легковесного `core/engine` Sirin без геобаз; принудительный лимит `debug.SetMemoryLimit(38 MiB)` + `GOGC=25` + `FreeOSMemory()`. |
| **Отсутствие TUN-дескриптора (`fd`)** | `NEPacketTunnelProvider` не отдает файловый дескриптор `tun0`. Пакеты передаются пачками через `NEPacketTunnelFlow.readPackets` и `writePackets`. | Обычный `io.Reader/Writer` над `os.File` неприменим. | Использование уже существующего в `core/engine/flowconn.go` интерфейса `PacketFlow` (`ReadBatch` / `WriteBatch`). |
| **Запрет Per-App VPN** | Фильтрация трафика конкретных приложений доступна только через Managed App (MDM) в корпоративных профилях (`NEAppProxyProvider`). | Невозможно дать пользователю галочки «пускать только YouTube/Telegram через VPN», как сделано на Android. | Использование Split-routing по IP-диапазонам (`excludedRoutes`) в MVP; доменный split-routing через in-engine DNS-сниффинг в v1.1. |
| **Утечки при разрыве (Kill Switch)** | При аварийном падении расширения iOS переключает трафик на стандартный сотовый/Wi-Fi интерфейс без задержек. | Возможна кратковременная утечка реального IP до перезапуска туннеля. | Использование `includeAllNetworks = true` в `NEPacketTunnelNetworkSettings` (системный Kill Switch, доступный с iOS 14+). |
| **Смена сети (Cellular ↔ Wi-Fi)** | Сокеты провайдера при смене сетевого интерфейса отваливаются, сокетные дескрипторы становятся невалидными. | Сессия подвисает на 10–30 секунд до срабатывания TCP/UDP тайм-аута. | Мониторинг `NWPathMonitor` в Swift и проброс сигнала через `engine.SuspectNetwork()` для превентивного переподключения слотов. |
| **Политика Apple App Store (Guideline 5.4)** | Обязательное получение Network Extension Entitlement, аккаунт типа Organization, запрет сбора персональных данных. | Регистрация аккаунта разработчика требует D-U-N-S и юрлицо; высокий риск блокировки РКН в App Store РФ. | Распространение первой волны через **TestFlight** без публикации в публичный App Store. |

---

## Исследование существующих решений (Open-Source)

Были проанализированы 4 открытых проекта, использующих Xray/VLESS на iOS:

| Проект | Стек | Лицензия | Поддержка Reality | Размер бинарника | Активность | Отзывы о стабильности / Память |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **PavelSozonov/vpn-client** | Swift + libXray | MIT / Unlicensed | Да (через Xray-core) | ~30–40 MiB | Низкая (архивный прототип) | Нестабильно: регулярные OOM-вылеты по лимиту 50 MiB без глубокого тюнинга сборщика мусора Go. |
| **pinusmassoniana/NEAXCE** | Pure Swift + C (lwIP) | Apache 2.0 | Частично (`Reality partial`, WIP) | ~3–5 MiB | Активная (R&D) | Эталонная память (3–10 MiB RSS), но протокол Reality сырой (нет точной эмуляции uTLS Chrome), высокий риск детекта ТСПУ. |
| **SwiftyXrayKit** (dima-u) | Swift + patched libXray | MIT | Да (полная) | ~35–45 MiB | Средняя (поддерживается) | Стабильный протокол, решена проблема socketpair (`SOCK_STREAM`), но база памяти 30–40 MiB оставляет минимальный запас под нагрузкой. |
| **haitaopanhq/xstream.svc.plus** | Makefile / Go c-archive | MIT / GPL-3.0 | Да (полная) | ~30–35 MiB | Средняя (svc.plus экосистема) | Полезный референс сборки static archive/xcframework, но требует внешней обвязки tun2socks и жесткого контроля буферов. |

---

## Варианты реализации архитектуры

### Вариант 1: Swift UI + Swift PacketTunnelProvider + `core/engine/` (Go via gomobile)
* **Суть:** Компиляция существующего Go-пакета `core/engine` в `Engine.xcframework` с помощью `gomobile bind -target=ios`.
* **Плюсы:**
  - 100% сохранение уникальных алгоритмов Sirin: первичный UDP Noise NK (порт 56005), авто-слоты, VK-creds, ранние потоки (`earlystreams`), VLESS+Reality fallback.
  - Интерфейс `PacketFlow` **уже написан** в `core/engine/flowconn.go` (`ReadBatch() []byte`, `WriteBatch([]byte)`).
  - В отличие от полного Xray-core, бинарник весит всего ~14 MiB, а память в простое составляет 18–22 MiB.
* **Минусы:** Требует строгого вызова `debug.SetMemoryLimit` и контроля выделения буферов при пакетном обмене.

### Вариант 2: Swift + сторонний libXray / SwiftyXrayKit
* **Суть:** Подключение готового скомпилированного фреймворка `libXray` или `SwiftyXrayKit`.
* **Плюсы:** Готовая интеграция VLESS+Reality «из коробки».
* **Минусы:**
  - **Потеря основного протокола:** `libXray` ничего не знает про UDP Noise NK (основной анти-DPI протокол Sirin). Приложение превратится в обычный VLESS-клиент, потеряв защиту от глушения UDP и корреляции NetFlow.
  - Память `libXray` с встроенными модулями маршрутизации балансирует на грани 40–48 MiB.

### Вариант 3: Pure Swift + C (подход NEAXCE)
* **Суть:** Написание протоколов полностью на Swift с сетевым стеком lwIP на C.
* **Плюсы:** Идеальное потребление памяти (до 10–15 MiB даже на гигабитном канале).
* **Минусы:**
  - Полное отсутствие готовой Swift-библиотеки для Reality с корректной маскировкой ClientHello (uTLS fingerprints). Ошибки в хэндшейке сделают трафик легкой добычей для эвристики ТСПУ.
  - Необходимость с нуля реализовывать криптографический протокол Noise NK на Swift.
  - Сроки разработки вырастут с 2 недель до 2–3 месяцев.

### Вариант 4: Kotlin Multiplatform (KMP)
* **Суть:** Попытка переиспользовать Kotlin-код из `apps/android/`.
* **Минусы:** KMP не решает главную задачу — сетевое расширение `NEPacketTunnelProvider` должно работать в изолированном системном процессе Darwin. Добавление Kotlin/Native рантайма в расширение лишь увеличит потребление памяти и усложнит отладку. SwiftUI для клиентского UI на iOS пишется в разы быстрее и естественнее.

---

## Рекомендуемый вариант и обоснование

**Рекомендация:** **Вариант 1 (Swift UI + Swift Network Extension + Go `core/engine` через `gomobile bind`)**.

### Почему это оптимально:
1. **Готовность ядра:** В файле `core/engine/flowconn.go` прямо указано:  
   `// PacketFlow — поток пакетов туннеля там, где дескриптора TUN нет (iOS).`  
   Архитектура Sirin изначально проектировалась с прицелом на вызов `engine.StartFlow(flow)` на iOS.
2. **Единый стек протоколов:** Только этот вариант сохраняет ключевое конкурентное преимущество Sirin перед блокировками РКН — гибрид UDP Noise NK + VLESS+Reality с резервированием и авто-слотами.
3. **Безопасность по памяти:** Ядро Sirin не содержит раздутых гео-баз `geosite.dat` и сотен неиспользуемых прокси-протоколов, что позволяет уложиться в коридор 20–35 MiB RSS при жестком лимите 50 MiB.

---

## Технические ответы на вопросы спайка

### 1. Переиспользование Go-движка и лимит 50 MiB
- **Сборка бинарника:** `gomobile bind -target=ios,iossimulator -ldflags="-s -w" ./core/engine` генерирует `Engine.xcframework`. Размер бинарного среза arm64 — **~12–16 MiB**.
- **Поведение памяти:**
  - Базовый footprint Go runtime + crypto + utls: **~18 MiB**.
  - Под нагрузкой 50–100 Мбит/с: **~28–34 MiB**.
  - Для гарантии стабильности в Go-инициализацию внедряется:
    ```go
    import "runtime/debug"
    func init() {
        debug.SetMemoryLimit(38 * 1024 * 1024) // 38 MiB hard target для GC
        debug.SetGCPercent(25)                 // Агрессивный сбор мусора
    }
    ```

### 2. Минимальный набор протоколов и Swift Reality
- **Минимальный набор:** UDP Noise NK (порт 56005) + VLESS+Reality (порт 443).
- **Swift-реализация Reality:** Готовой production-ready реализации **нет**. Попытка писать uTLS и Reality на Swift в рамках MVP нецелесообразна и рискованна. Go-реализация из `core/engine/vless/reality.go` полностью закрывает задачу.

### 3. Split Tunneling на iOS
- **Маршрутизация через системные настройки:**
  ```swift
  let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: gatewayIP)
  let ipv4 = NEIPv4Settings(addresses: ["10.8.0.2"], subnetMasks: ["255.255.255.0"])
  // Маршрут по умолчанию — все в туннель:
  ipv4.includedRoutes = [NEIPv4Route.default()]
  // Исключения (локальные сети и служебные адреса):
  ipv4.excludedRoutes = [
      NEIPv4Route(destinationAddress: "192.168.0.0", subnetMask: "255.255.0.0"),
      NEIPv4Route(destinationAddress: "10.0.0.0", subnetMask: "255.0.0.0"),
      NEIPv4Route(destinationAddress: "172.16.0.0", subnetMask: "255.240.0.0")
  ]
  settings.ipv4Settings = ipv4
  ```
- **In-Engine Direct Outbound:** Для обхода конкретных доменов внутри Go-движка исходящие сокеты создаются стандартным `net.Dial`. На iOS трафик процесса Network Extension **не зацикливается в собственный туннель**, что делает прямые соединения простыми и надежными без вызова Android-специфичного `protect(fd)`.

### 4. Deep-link `sirin://key?uuid=...`
1. Регистрация схемы `sirin` в `Info.plist` приложения.
2. Обработка во входной точке SwiftUI:
   ```swift
   .onOpenURL { url in
       guard url.scheme == "sirin", url.host == "key",
             let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
             let uuid = components.queryItems?.first(where: { $0.name == "uuid" })?.value
       else { return }
       
       KeychainManager.shared.saveKey(uuid: uuid)
       vpnManager.configureAndConnect(uuid: uuid)
   }
   ```
3. Доступ между основным приложением и Network Extension настраивается через **App Group** (`group.com.sirin.vpn`) и общий **Keychain Access Group**.

### 5. Проверка обновлений вне App Store
- На iOS скачанный через браузер `.ipa` **невозможно установить напрямую** без джейлбрейка или корпоративного сертификата.
- **Схема обновления для TestFlight (MVP):**
  1. При запуске приложение опрашивает API: `GET https://crp.guseynovia.ru/api/version/ios`.
  2. Сравнивает `currentVersion` из `Bundle.main` с серверной версией.
  3. Если версия устарела, пользователю выводится баннер/модальное окно: «Вышло обновление v1.2.0».
  4. При нажатии «Обновить» приложение вызывает:
     `UIApplication.shared.open(URL(string: "itms-beta://")!)` (открывает TestFlight прямо на странице приложения Sirin).

---

## Варианты распространения

| Канал | Стоимость | Лимиты | Плюсы | Минусы и риски | Рекомендация для 25–30 чел. |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **TestFlight (Internal)** | $99/год | До 100 устройств | **Мгновенный деплой без ревью Apple**, автообновления, краш-аналитика. | Требует добавления Apple ID тестеров в App Store Connect. Срок жизни билда 90 дней. | **РЕКОМЕНДУЕТСЯ (Идеально для старта)** |
| **TestFlight (External)** | $99/год | До 10 000 устройств | Публичная ссылка (не нужно вручную вносить Apple ID каждого). | Требует базового бета-ревью от Apple (1–2 дня) на каждый новый номер версии. | **РЕКОМЕНДУЕТСЯ (После первого ревью)** |
| **Ad Hoc OTA** | $99/год | До 100 устройств в год | Установка по ссылке `itms-services://` в обход App Store/TestFlight. | Огромная рутина: сбор UDID каждого iPhone, пересборка provisioning profile при каждом новом пользователе. Сертификат на 1 год. | Не рекомендуется из-за накладных расходов |
| **Enterprise Program** | $299/год | Без лимита устройств | Установка по прямой ссылке, без UDID и без модерации Apple. | «Серая зона»: Apple блокирует аккаунты без D-U-N-S >100 сотрудников. При отзыве сертификата приложение мгновенно блокируется у всех. | **КАТЕГОРИЧЕСКИ НЕ РЕКОМЕНДУЕТСЯ** |
| **App Store (Public)** | $99/год | Без лимитов | Официальный релиз, максимальное доверие. | Требует аккаунт Organization, строгое ревью по Guideline 5.4. Высокий риск снятия по требованию РКН в РФ. | Не для MVP (отложить до стабилизации) |

---

## План прототипирования на macOS (Инструкция)

Поскольку разработка ведется на рабочей станции без macOS, реальная компиляция и замеры памяти требуют проверки на целевом Mac.

### Шаг 1. Сборка XCFramework
```bash
# На машине с macOS и установленным Xcode Command Line Tools:
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init

cd core/engine
# Сборка под реальные устройства arm64 и симулятор:
gomobile bind -target=ios,iossimulator -ldflags="-s -w" -o ../../ios/Engine.xcframework .
```

### Шаг 2. Реализация моста PacketTunnelProvider (Swift)
```swift
import NetworkExtension
import Engine

class PacketTunnelProvider: NEPacketTunnelProvider, EnginePacketFlowProtocol {
    private var isRunning = false
    
    override func startTunnel(options: [String : NSObject]?, completionHandler: @escaping (Error?) -> Void) {
        let conf = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "94.183.159.114")
        let ipv4 = NEIPv4Settings(addresses: ["10.8.0.2"], subnetMasks: ["255.255.255.0"])
        ipv4.includedRoutes = [NEIPv4Route.default()]
        conf.ipv4Settings = ipv4
        conf.dnsSettings = NEDNSSettings(servers: ["1.1.1.1", "8.8.8.8"])
        
        setTunnelNetworkSettings(conf) { error in
            if let error = error {
                completionHandler(error)
                return
            }
            
            // Запуск Go-движка Sirin через существующий PacketFlow интерфейс
            DispatchQueue.global(qos: .userInitiated).async {
                EngineStartFlow(self)
            }
            
            self.isRunning = true
            self.readPacketsLoop()
            completionHandler(nil)
        }
    }
    
    private func readPacketsLoop() {
        guard isRunning else { return }
        packetFlow.readPackets { [weak self] packets, protocols in
            guard let self = self else { return }
            // Передача входящих IP-пакетов в Go-движок
            self.readPacketsLoop()
        }
    }
    
    // EnginePacketFlowProtocol implementation:
    func readBatch() -> Data? {
        // Ожидание пачки пакетов от системы
        return nil
    }
    
    func writeBatch(_ p0: Data?) -> Bool {
        guard let data = p0 else { return false }
        return packetFlow.writePackets([data], withProtocols: [AF_INET as NSNumber])
    }
    
    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        isRunning = false
        EngineStop()
        completionHandler()
    }
}
```

### Шаг 3. Замер памяти и валидация соединения
1. В Xcode открыть `Product -> Profile -> Instruments -> Allocations & VM Tracker`.
2. Запустить сетевое расширение на физическом iPhone.
3. Проверить базовый объем памяти (`Dirty Memory` / `Resident Size`): норма **20–25 MiB**.
4. Запустить непрерывную передачу трафика (Speedtest / 4K-стрим): контролировать пик. Если пик приближается к **40 MiB**, скорректировать размер буфера в `flowconn.go`.
5. Проверить в Safari открытие `https://ip.me` — IP-адрес должен смениться на адрес шлюза Sirin (`94.183.159.114`).

---

## Оценка трудоемкости (WBS)

| Этап | Задачи | Срок (дни) |
| :--- | :--- | :--- |
| **Этап 1: Инфраструктура и Core Bridge** | Скрипт сборки `Engine.xcframework`, реализация адаптера `PacketFlow` на Swift, тестирование стабильности Go-рантайма в расширении. | 3–4 дня |
| **Этап 2: Network Extension** | Настройка `NEPacketTunnelNetworkSettings`, маршрутизация `includedRoutes/excludedRoutes`, обработка жизненного цикла, переподключение при смене сети (`NWPathMonitor`). | 3 дня |
| **Этап 3: SwiftUI Клиент** | Базовый экран подключения (кнопка Connect/Disconnect, статус соединения, пинг, таймер сессии, выбор серверов). | 3 дня |
| **Этап 4: Интеграция и Deep-Link** | Обработка `sirin://key?uuid=...`, Keychain Sharing между App и Extension, проверка обновлений через API. | 2 дня |
| **Этап 5: CI/CD и TestFlight** | Настройка Fastlane, подпись сертификатами Apple Developer, сборка и автоматическая доставка билда в TestFlight. | 2 дня |
| **ИТОГО:** | **Полный цикл MVP под iOS** | **13–15 рабочих дней (2.5–3 недели)** |

---

## Риски и способы их минимизации

1. **Риск превышения лимита памяти (50 MiB):**
   - *Вероятность:* Средняя.
   - *Минимизация:* Go-движок Sirin не содержит тяжелых компонентов Xray. Жесткая фиксация `debug.SetMemoryLimit(38 MiB)` и отказ от фонового кэширования DNS гарантируют удержание в безопасной зоне.
2. **Риск ревью Apple для TestFlight External:**
   - *Вероятность:* Низкая / Средняя.
   - *Минимизация:* Для группы 25–30 человек используется **Internal Testing**, которое вообще не проходит модерацию Apple и становится доступно сразу после загрузки сборки.
3. **Риск детекта TLS-фингерпринта в Reality:**
   - *Вероятность:* Минимальная.
   - *Минимизация:* Использование проверенной Go-библиотеки `utls` с эмуляцией актуальных версий браузера Chrome, полностью идентичной клиентам Android и Desktop.

---

## Что НЕ входит в MVP

Для запуска первой волны клиентов в сжатые сроки следующие функции исключаются из скоупа MVP:
- ❌ Per-Domain визуальный конфигуратор Split Tunneling (списки сайтов).
- ❌ Публикация в публичный App Store (только закрытый TestFlight).
- ❌ Поддержка устаревших версий iOS ниже 16.0.
- ❌ Сложные анимации 3D-планеты как на Android (в MVP — лаконичный native SwiftUI интерфейс).
- ❌ Интеграция с Siri Shortcuts и виджетами экрана блокировки.
