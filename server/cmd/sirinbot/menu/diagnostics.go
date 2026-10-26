package menu

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"sirin/server/internal/bot"
	"sirin/server/internal/bot/diagnostics"
	tele "gopkg.in/telebot.v3"
)

// diagRateLimiter ограничивает количество запусков диагностики (максимум 10 в час на пользователя).
type diagRateLimiter struct {
	mu      sync.Mutex
	history map[int64][]time.Time
}

func (l *diagRateLimiter) Allow(userID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)

	var filtered []time.Time
	for _, t := range l.history[userID] {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) >= 10 {
		l.history[userID] = filtered
		return false
	}

	l.history[userID] = append(filtered, now)
	return true
}

var defaultDiagLimiter = &diagRateLimiter{history: make(map[int64][]time.Time)}

// lastReportsCache хранит последний отчёт диагностики для каждого пользователя.
var (
	lastReportsMu sync.RWMutex
	lastReports   = make(map[int64]*diagnostics.DiagnosticReport)
)

// GetLastDiagnosticReport возвращает последний сохранённый отчёт диагностики пользователя.
func GetLastDiagnosticReport(userID int64) *diagnostics.DiagnosticReport {
	lastReportsMu.RLock()
	defer lastReportsMu.RUnlock()
	return lastReports[userID]
}

func setLastDiagnosticReport(userID int64, report *diagnostics.DiagnosticReport) {
	lastReportsMu.Lock()
	defer lastReportsMu.Unlock()
	lastReports[userID] = report
}

// StartDiagnosticsWizard начинает процесс диагностики.
func (m *Manager) StartDiagnosticsWizard(c tele.Context, edit bool) error {
	userID := c.Sender().ID

	// 1. Проверяем наличие ключа и устройств в bot_users
	uk, hasKey := m.bot.Storage().Get(userID)
	if !hasKey || uk == nil {
		return m.ShowDiagStep0KeyPrompt(c, edit)
	}

	devs := uk.GetDevices()
	if len(devs) == 0 {
		return m.ShowDiagStep0KeyPrompt(c, edit)
	}

	// Если устройство 1 — авто-выбор и переход к диагностике
	if len(devs) == 1 {
		return m.startDiagForDevice(c, edit, devs[0])
	}

	// Если устройств > 1 — экран выбора устройства
	return m.ShowDiagDeviceSelector(c, edit, devs)
}

// ShowDiagDeviceSelector отображает экран выбора устройства при наличии нескольких.
func (m *Manager) ShowDiagDeviceSelector(c tele.Context, edit bool, devs []bot.Device) error {
	m.fsm.Set(c.Sender().ID, "diag_select_device", "", 0, nil)

	text := "🩺 *Диагностика соединения — Выбор устройства*\n\n" +
		"У вас привязано несколько устройств. Какое устройство вы хотите проверить сейчас?"

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	for _, d := range devs {
		platformIcon := "📱"
		switch d.Platform {
		case "windows":
			platformIcon = "💻"
		case "router":
			platformIcon = "🌐"
		}
		btnLabel := fmt.Sprintf("%s %s", platformIcon, d.Label)
		btn := markup.Data(btnLabel, "diag", "dev", d.UUID)
		rows = append(rows, markup.Row(btn))
	}

	btnAdd := markup.Data("➕ Добавить новое устройство", "my", "add_device")
	btnBack := markup.Data("◀ В меню", "my", "home")
	rows = append(rows, markup.Row(btnAdd), markup.Row(btnBack))

	markup.Inline(rows...)
	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

func (m *Manager) startDiagForDevice(c tele.Context, edit bool, dev bot.Device) error {
	m.fsm.Set(c.Sender().ID, "diag_step2_issue", dev.UUID, 0, map[string]string{
		"uuid":         dev.UUID,
		"platform":     dev.Platform,
		"transport":    dev.Transport,
		"version":      dev.AppVersion,
		"device_label": dev.Label,
	})
	return m.ShowDiagStep2Issue(c, edit)
}

// ShowDiagStep0KeyPrompt шаг 0: ввод ключа (UUID) при отсутствии привязки.
func (m *Manager) ShowDiagStep0KeyPrompt(c tele.Context, edit bool) error {
	m.fsm.Set(c.Sender().ID, "diag_step0_key", "", 0, nil)

	text := "🩺 *Диагностика — Шаг 0: Ввод ключа доступа*\n\n" +
		"У вас пока нет привязанного ключа Сирин в этом чате.\n\n" +
		"Отправьте ваш ключ доступа (UUID) сообщением в этот чат или выполните команду /mykey.\n\n" +
		"📌 *Пример формата:*\n`30e382b8-a8e0-4640-bd8e-68313636bb89`"

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("◀ В главное меню", "my", "home")
	markup.Inline(markup.Row(btnCancel))

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// HandleDiagStep0KeyInput обрабатывает ввод UUID пользователем на шаге 0.
func (m *Manager) HandleDiagStep0KeyInput(c tele.Context, state *State, input string) error {
	uuid := strings.TrimSpace(input)
	if strings.Contains(uuid, "uuid=") {
		parts := strings.Split(uuid, "uuid=")
		if len(parts) > 1 {
			uuid = strings.Split(parts[1], "&")[0]
		}
	}

	client, ok := m.bot.ClientsStorage().Get(uuid)
	if !ok || client == nil {
		markup := &tele.ReplyMarkup{}
		btnCancel := markup.Data("◀ Отмена", "my", "home")
		markup.Inline(markup.Row(btnCancel))
		return c.Send("❌ *Ключ не найден в базе данных сервера.*\n\nПожалуйста, проверьте правильность копирования UUID или обратитесь к администратору.", markup, tele.ModeMarkdown)
	}

	// Сохраняем устройство в профиль пользователя
	dev := bot.Device{
		UUID:       client.UUID,
		Label:      client.Name,
		Platform:   "android",
		Transport:  "unknown",
		AddedAt:    time.Now().Unix(),
	}
	_ = m.bot.Storage().AddDevice(c.Sender().ID, dev)

	_ = c.Send(fmt.Sprintf("✅ *Ключ успешно привязан!*\nУстройство: *%s*", dev.Label), tele.ModeMarkdown)
	return m.startDiagForDevice(c, false, dev)
}

// ShowDiagStep1Platform шаг 1: выбор платформы.
func (m *Manager) ShowDiagStep1Platform(c tele.Context, edit bool, uuid string) error {
	m.fsm.Set(c.Sender().ID, "diag_step1_platform", uuid, 0, map[string]string{
		"uuid": uuid,
	})

	text := fmt.Sprintf(
		"🩺 *Диагностика соединения — Шаг 1 из 4*\n\n"+
			"🔑 Ключ: `%s`\n\n"+
			"На каком устройстве вы пользуетесь Сирин?",
		diagnostics.MaskUUID(uuid),
	)

	markup := &tele.ReplyMarkup{}
	btnAndroid := markup.Data("📱 Android", "diag", "p", "android")
	btnWindows := markup.Data("💻 Windows", "diag", "p", "windows")
	btnRouter := markup.Data("🌐 Роутер (Keenetic/OpenWrt)", "diag", "p", "router")
	btnOther := markup.Data("❓ Другое устройство", "diag", "p", "other")
	btnCancel := markup.Data("◀ В меню", "my", "home")

	markup.Inline(
		markup.Row(btnAndroid, btnWindows),
		markup.Row(btnRouter),
		markup.Row(btnOther),
		markup.Row(btnCancel),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowDiagStep2Issue шаг 2: тип проблемы.
func (m *Manager) ShowDiagStep2Issue(c tele.Context, edit bool) error {
	state, ok := m.fsm.Get(c.Sender().ID)
	if !ok {
		return m.StartDiagnosticsWizard(c, edit)
	}
	state.Action = "diag_step2_issue"
	m.fsm.Set(c.Sender().ID, state.Action, state.ClientID, state.MessageID, state.Data)

	platformLabel := formatPlatform(state.Data["platform"])
	text := fmt.Sprintf(
		"🩺 *Диагностика соединения — Шаг 2 из 4*\n\n"+
			"📱 Устройство: *%s*\n\n"+
			"С чем именно возникла проблема?",
		platformLabel,
	)

	markup := &tele.ReplyMarkup{}
	btnNoConnect := markup.Data("🔴 Не подключается вообще", "diag", "i", "no_connect")
	btnSlow := markup.Data("🐢 Медленная скорость / обрывы", "diag", "i", "slow")
	btnBlocked := markup.Data("🚫 Не открываются YouTube / Insta", "diag", "i", "sites_blocked")
	btnKey := markup.Data("🔑 Ошибка ключа / подписки", "diag", "i", "key_issue")
	btnOther := markup.Data("❓ Другая проблема", "diag", "i", "other")
	btnBack := markup.Data("◀ Назад", "diag", "back", "1")

	markup.Inline(
		markup.Row(btnNoConnect),
		markup.Row(btnBlocked),
		markup.Row(btnSlow),
		markup.Row(btnKey, btnOther),
		markup.Row(btnBack),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowDiagStep3IPCheck шаг 3: проверка внешнего IP.
func (m *Manager) ShowDiagStep3IPCheck(c tele.Context, edit bool) error {
	state, ok := m.fsm.Get(c.Sender().ID)
	if !ok {
		return m.StartDiagnosticsWizard(c, edit)
	}
	state.Action = "diag_step3_ip"
	m.fsm.Set(c.Sender().ID, state.Action, state.ClientID, state.MessageID, state.Data)

	text := "🩺 *Диагностика соединения — Шаг 3 из 4*\n\n" +
		"🌐 *Проверка защищённого туннеля (IP-адрес)*\n\n" +
		"Чтобы проверить, проходит ли трафик через сервер Сирин:\n" +
		"1. Включите подключение в приложении Сирин;\n" +
		"2. Откройте сайт [api.ipify.org](https://api.ipify.org) или [ip.me](https://ip.me);\n" +
		"3. Отправьте скопированный IP сообщением в этот чат.\n\n" +
		"Либо нажмите *«Пропустить»*, если не можете сейчас проверить IP."

	markup := &tele.ReplyMarkup{}
	btnSkip := markup.Data("⏭️ Пропустить проверку IP", "diag", "ip", "skip")
	btnBack := markup.Data("◀ Назад", "diag", "back", "2")

	markup.Inline(
		markup.Row(btnSkip),
		markup.Row(btnBack),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// HandleDiagStep3IPInput обрабатывает ввод IP пользователем.
func (m *Manager) HandleDiagStep3IPInput(c tele.Context, state *State, input string) error {
	ipStr := strings.TrimSpace(input)
	parsed := net.ParseIP(ipStr)
	if parsed == nil {
		markup := &tele.ReplyMarkup{}
		btnSkip := markup.Data("⏭️ Пропустить проверку IP", "diag", "ip", "skip")
		markup.Inline(markup.Row(btnSkip))
		return c.Send("⚠️ Не удалось распознать IP-адрес. Введите корректный IPv4-адрес или нажмите *«Пропустить»*.", markup, tele.ModeMarkdown)
	}

	state.Data["client_ip"] = ipStr
	m.fsm.Set(c.Sender().ID, "diag_step4_version_transport", state.ClientID, 0, state.Data)
	return m.ShowDiagStep4VersionTransport(c, false)
}

// ShowDiagStep4VersionTransport шаг 4: выбор версии и транспорта.
func (m *Manager) ShowDiagStep4VersionTransport(c tele.Context, edit bool) error {
	state, ok := m.fsm.Get(c.Sender().ID)
	if !ok {
		return m.StartDiagnosticsWizard(c, edit)
	}
	state.Action = "diag_step4_version_transport"
	m.fsm.Set(c.Sender().ID, state.Action, state.ClientID, state.MessageID, state.Data)

	platform := state.Data["platform"]
	currentVer := state.Data["version"]
	if currentVer == "" {
		currentVer = "не выбрана"
	}
	currentTrans := state.Data["transport"]
	if currentTrans == "" {
		currentTrans = "не выбран"
	} else {
		currentTrans = strings.ToUpper(currentTrans)
	}

	ipStatus := "не указан (пропущено)"
	if state.Data["client_ip"] != "" {
		ipStatus = diagnostics.MaskIP(state.Data["client_ip"])
	}

	text := fmt.Sprintf(
		"🩺 *Диагностика соединения — Шаг 4 из 4*\n\n"+
			"📱 Устройство: *%s*\n"+
			"❓ Проблема: *%s*\n"+
			"🌐 Внешний IP: *%s*\n\n"+
			"Параметры подключения:\n"+
			"• Версия: *%s*\n"+
			"• Транспорт: *%s*\n\n"+
			"Укажите версию приложения и протокол (если знаете), затем нажмите *«Запустить диагностику»*:",
		formatPlatform(platform),
		formatIssue(state.Data["issue"]),
		ipStatus,
		currentVer,
		currentTrans,
	)

	markup := &tele.ReplyMarkup{}

	// Кнопки версий в зависимости от платформы
	var verRows []tele.Row
	switch platform {
	case "android":
		btn1 := markup.Data("Android 1.3.5 (Новая)", "diag", "v", "1.3.5")
		btn2 := markup.Data("Android 1.3.4", "diag", "v", "1.3.4")
		btnUnknown := markup.Data("Версия: Не знаю", "diag", "v", "unknown")
		verRows = append(verRows, markup.Row(btn1, btn2), markup.Row(btnUnknown))
	case "windows":
		btn1 := markup.Data("Windows 1.0.0", "diag", "v", "1.0.0")
		btnUnknown := markup.Data("Версия: Не знаю", "diag", "v", "unknown")
		verRows = append(verRows, markup.Row(btn1, btnUnknown))
	case "router":
		btn1 := markup.Data("Router 1.3.1", "diag", "v", "1.3.1")
		btn2 := markup.Data("Router 1.2.0", "diag", "v", "1.2.0")
		btnUnknown := markup.Data("Версия: Не знаю", "diag", "v", "unknown")
		verRows = append(verRows, markup.Row(btn1, btn2), markup.Row(btnUnknown))
	default:
		btnUnknown := markup.Data("Версия: Не знаю", "diag", "v", "unknown")
		verRows = append(verRows, markup.Row(btnUnknown))
	}

	// Кнопки транспорта
	btnVless := markup.Data("⚡ VLESS", "diag", "t", "vless")
	btnXhttp := markup.Data("🌐 XHTTP", "diag", "t", "xhttp")
	btnUDP := markup.Data("📡 UDP", "diag", "t", "udp")
	btnTransUnknown := markup.Data("Транспорт: Не знаю", "diag", "t", "unknown")
	transRow1 := markup.Row(btnVless, btnXhttp, btnUDP)
	transRow2 := markup.Row(btnTransUnknown)

	// Кнопка запуска
	btnRun := markup.Data("🚀 Запустить диагностику", "diag", "run")
	btnBack := markup.Data("◀ Назад", "diag", "back", "3")
	actionRow := markup.Row(btnRun)
	navRow := markup.Row(btnBack)

	var allRows []tele.Row
	allRows = append(allRows, verRows...)
	allRows = append(allRows, transRow1, transRow2, actionRow, navRow)

	markup.Inline(allRows...)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ExecuteDiagnostics запускает выполнение движка диагностики и выводит интерактивный отчёт.
func (m *Manager) ExecuteDiagnostics(c tele.Context) error {
	userID := c.Sender().ID

	// Проверка rate limit: 10 проверок в час
	if !defaultDiagLimiter.Allow(userID) {
		RespondAlert(c, "Превышен лимит запусков диагностики (максимум 10 в час). Попробуйте позже.")
		return nil
	}

	state, ok := m.fsm.Get(userID)
	uuid := ""
	if ok && state.ClientID != "" {
		uuid = state.ClientID
	} else if uk, has := m.bot.Storage().Get(userID); has && uk != nil {
		uuid = uk.UUID
	}

	client, _ := m.bot.ClientsStorage().Get(uuid)
	user, _ := m.bot.Storage().Get(userID)

	_ = EditOrSend(c, "⏳ *Выполняется комплексная диагностика...*\n\n_Проверяем ключ, API-сервер, параметры маршрутизации и релеи..._", &tele.ReplyMarkup{})

	input := diagnostics.ClientInput{
		Platform:   state.Data["platform"],
		IssueType:  state.Data["issue"],
		ClientIP:   state.Data["client_ip"],
		AppVersion: state.Data["version"],
		Transport:  state.Data["transport"],
	}

	deps := diagnostics.Dependencies{
		APIURL:     m.bot.Config().APIURL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		Clients:    m.bot.ClientsStorage(),
		Storage:    m.bot.Storage(),
		Relays:     m.bot.RelaysStorage(),
		Releases:   defaultReleaseCache,
		TCPProbe:   diagnostics.DefaultTCPProbe,
		IsAdmin:    m.bot.IsAdmin(userID),
	}

	engine := diagnostics.NewEngine(deps)
	report := engine.Run(context.Background(), client, user, userID, input)

	setLastDiagnosticReport(userID, &report)

	// Обновляем метку времени диагностики устройства и его параметры
	if uuid != "" {
		_ = m.bot.Storage().UpdateDeviceDiag(userID, uuid, time.Now())
		if user != nil {
			if dev, ok := user.GetDevice(uuid); ok && dev != nil {
				updated := *dev
				if input.Platform != "" {
					updated.Platform = input.Platform
				}
				if input.Transport != "" && input.Transport != "unknown" {
					updated.Transport = input.Transport
				}
				if input.AppVersion != "" && input.AppVersion != "unknown" {
					updated.AppVersion = input.AppVersion
				}
				_ = m.bot.Storage().AddDevice(userID, updated)
			}
		}
	}

	text := report.FormatText(m.bot.IsAdmin(userID))

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Авто-фиксы
	hasResetFix := false
	for _, fix := range report.GetAutoFixes() {
		if fix.ID == "reset_relay_nl" {
			hasResetFix = true
			rows = append(rows, markup.Row(markup.Data(fix.Title, "diag", "fix", fix.ID)))
		} else if fix.ID == "link_telegram" {
			rows = append(rows, markup.Row(markup.Data(fix.Title, "diag", "fix", fix.ID)))
		} else if fix.ID == "update_app" && fix.Description != "" {
			rows = append(rows, markup.Row(markup.URL(fix.Title, fix.Description)))
		}
	}

	// Опциональная кнопка "Попробовать Direct NL", если клиент на релее, но авто-фикс не был добавлен
	if !hasResetFix && client != nil && client.RelayID > 0 {
		btnDirectNL := markup.Data("🔧 Попробовать Direct NL", "diag", "fix", "reset_relay_nl")
		rows = append(rows, markup.Row(btnDirectNL))
	}

	// Кнопка создания тикета
	btnTicket := markup.Data("✉️ Создать тикет в поддержку", "diag", "ticket")
	btnRestart := markup.Data("🔄 Проверить снова", "diag", "restart")
	btnMenu := markup.Data("◀ В меню", "my", "home")

	rows = append(rows, markup.Row(btnTicket))
	rows = append(rows, markup.Row(btnRestart, btnMenu))

	markup.Inline(rows...)
	return EditOrSend(c, text, markup)
}

// HandleDiagCallback диспетчеризирует callback-кнопки диагностики.
func (m *Manager) HandleDiagCallback(c tele.Context, action string, args []string) error {
	RespondSafe(c)
	userID := c.Sender().ID

	switch action {
	case "start":
		return m.StartDiagnosticsWizard(c, true)

	case "dev": // Выбор конкретного устройства
		if len(args) > 0 {
			targetUUID := args[0]
			uk, hasKey := m.bot.Storage().Get(userID)
			if hasKey && uk != nil {
				if d, ok := uk.GetDevice(targetUUID); ok {
					return m.startDiagForDevice(c, true, *d)
				}
			}
			return m.ShowDiagStep1Platform(c, true, targetUUID)
		}

	case "p": // Выбор платформы
		if len(args) > 0 {
			state, ok := m.fsm.Get(userID)
			if !ok {
				return m.StartDiagnosticsWizard(c, true)
			}
			state.Data["platform"] = args[0]
			m.fsm.Set(userID, "diag_step2_issue", state.ClientID, 0, state.Data)
			return m.ShowDiagStep2Issue(c, true)
		}

	case "i": // Выбор проблемы
		if len(args) > 0 {
			state, ok := m.fsm.Get(userID)
			if !ok {
				return m.StartDiagnosticsWizard(c, true)
			}
			state.Data["issue"] = args[0]
			m.fsm.Set(userID, "diag_step3_ip", state.ClientID, 0, state.Data)
			return m.ShowDiagStep3IPCheck(c, true)
		}

	case "ip": // Пропуск ввода IP
		if len(args) > 0 && args[0] == "skip" {
			state, ok := m.fsm.Get(userID)
			if !ok {
				return m.StartDiagnosticsWizard(c, true)
			}
			state.Data["client_ip"] = ""
			m.fsm.Set(userID, "diag_step4_version_transport", state.ClientID, 0, state.Data)
			return m.ShowDiagStep4VersionTransport(c, true)
		}

	case "v": // Выбор версии
		if len(args) > 0 {
			state, ok := m.fsm.Get(userID)
			if !ok {
				return m.StartDiagnosticsWizard(c, true)
			}
			state.Data["version"] = args[0]
			m.fsm.Set(userID, "diag_step4_version_transport", state.ClientID, 0, state.Data)
			return m.ShowDiagStep4VersionTransport(c, true)
		}

	case "t": // Выбор транспорта
		if len(args) > 0 {
			state, ok := m.fsm.Get(userID)
			if !ok {
				return m.StartDiagnosticsWizard(c, true)
			}
			state.Data["transport"] = args[0]
			m.fsm.Set(userID, "diag_step4_version_transport", state.ClientID, 0, state.Data)
			return m.ShowDiagStep4VersionTransport(c, true)
		}

	case "back": // Навигация назад
		if len(args) > 0 {
			step := args[0]
			switch step {
			case "1":
				state, _ := m.fsm.Get(userID)
				uuid := ""
				if state != nil {
					uuid = state.ClientID
				}
				return m.ShowDiagStep1Platform(c, true, uuid)
			case "2":
				return m.ShowDiagStep2Issue(c, true)
			case "3":
				return m.ShowDiagStep3IPCheck(c, true)
			}
		}

	case "run": // Запуск диагностики
		return m.ExecuteDiagnostics(c)

	case "restart": // Перезапуск
		return m.StartDiagnosticsWizard(c, true)

	case "fix": // Применение авто-фикса
		if len(args) > 0 {
			fixID := args[0]
			return m.applyDiagFix(c, fixID)
		}

	case "ticket": // Переход к созданию тикета с отчётом
		return m.PromptCreateTicketFromDiag(c)
	}

	return nil
}

func (m *Manager) applyDiagFix(c tele.Context, fixID string) error {
	userID := c.Sender().ID
	uk, hasKey := m.bot.Storage().Get(userID)

	switch fixID {
	case "reset_relay_nl":
		if !hasKey || uk == nil || uk.UUID == "" {
			RespondAlert(c, "Ключ не найден")
			return nil
		}
		_, err := m.bot.ClientsStorage().SetRelayID(uk.UUID, 0)
		if err != nil {
			RespondAlert(c, "Ошибка сброса релея: "+err.Error())
			return nil
		}
		RespondAlert(c, "✅ Переключено на прямой сервер NL (Нидерланды)!")
		return m.ExecuteDiagnostics(c)

	case "link_telegram":
		report := GetLastDiagnosticReport(userID)
		if report == nil || report.ClientUUID == "" {
			RespondAlert(c, "Не найден ключ для привязки")
			return nil
		}
		client, ok := m.bot.ClientsStorage().Get(report.ClientUUID)
		if !ok || client == nil {
			RespondAlert(c, "Клиент не найден в базе")
			return nil
		}
		err := m.bot.Storage().Set(bot.UserKey{
			TelegramID: userID,
			UUID:       client.UUID,
			Name:       client.Name,
			CreatedAt:  time.Now().Unix(),
		})
		if err != nil {
			RespondAlert(c, "Ошибка привязки: "+err.Error())
			return nil
		}
		RespondAlert(c, "✅ Ключ привязан к вашему аккаунту Telegram!")
		return m.ExecuteDiagnostics(c)
	}

	return nil
}

// PromptCreateTicketFromDiag подготавливает создание тикета на основе диагностики.
func (m *Manager) PromptCreateTicketFromDiag(c tele.Context) error {
	userID := c.Sender().ID
	m.fsm.Set(userID, "ticket_diag_create", "", 0, nil)

	text := "✉️ *Создание тикета в поддержку*\n\n" +
		"Контекст диагностики и параметры подключения автоматически прикреплены к тикету.\n\n" +
		"Напишите кратко, что происходит (или нажмите *«Отправить как есть»*):"

	markup := &tele.ReplyMarkup{}
	btnSendAsIs := markup.Data("📤 Отправить как есть", "ticket", "send_diag")
	btnCancel := markup.Data("◀ Отмена", "my", "home")
	markup.Inline(
		markup.Row(btnSendAsIs),
		markup.Row(btnCancel),
	)

	return EditOrSend(c, text, markup)
}

func formatPlatform(p string) string {
	switch p {
	case "android":
		return "Android"
	case "windows":
		return "Windows"
	case "router":
		return "Роутер"
	case "other":
		return "Другое"
	default:
		return "Не выбрано"
	}
}

func formatIssue(issue string) string {
	switch issue {
	case "no_connect":
		return "Не подключается"
	case "slow":
		return "Медленная скорость"
	case "sites_blocked":
		return "Не открываются сайты / YouTube"
	case "key_issue":
		return "Проблема с ключом"
	case "other":
		return "Другое"
	default:
		return "Не указано"
	}
}
