package menu

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"sirin/server/internal/bot"
	"sirin/server/internal/relays"
	tele "gopkg.in/telebot.v3"
)

// ShowRelaysMenu отображает экран «📡 Релеи» со списком узлов и их загрузкой.
func (m *Manager) ShowRelaysMenu(c tele.Context, edit bool) error {
	m.fsm.Clear(c.Sender().ID)

	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище релеев не инициализировано.")
	}

	allClients := m.bot.ClientsStorage().List()
	directCount := relays.CountActiveOnRelay(allClients, 0)
	list := rs.GetAll()

	var sb strings.Builder
	sb.WriteString("📡 *Управление релеями*\n")
	sb.WriteString("─────────────────────\n\n")

	sb.WriteString("🌐 *Direct NL (Нидерланды)* `(id=0)`\n")
	sb.WriteString(fmt.Sprintf("   👥 *%d* / ∞  _(без лимита)_\n\n", directCount))

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	btnClientsDirect := markup.Data(fmt.Sprintf("👥 Клиенты Direct (%d)", directCount), "relay", "clients", "0")
	rows = append(rows, markup.Row(btnClientsDirect))

	for _, r := range list {
		cnt := relays.CountActiveOnRelay(allClients, r.ID)
		statusEmoji := "✅ активен"
		if !r.Enabled {
			statusEmoji = "⛔ отключён"
		}
		left := r.MaxClients - cnt
		if left < 0 {
			left = 0
		}

		sb.WriteString(fmt.Sprintf("📡 *%s* — %s\n", r.Name, statusEmoji))
		sb.WriteString(fmt.Sprintf("   🆔 ID: `%d` · 🌐 `%s`\n", r.ID, r.Host))
		sb.WriteString(fmt.Sprintf("   👥 *%d* / %d  _(осталось %d мест)_\n\n", cnt, r.MaxClients, left))

		btnCard := markup.Data(fmt.Sprintf("📡 %s", r.Name), "relay", "view", strconv.Itoa(r.ID))
		btnClients := markup.Data(fmt.Sprintf("👥 %d/%d", cnt, r.MaxClients), "relay", "clients", strconv.Itoa(r.ID))
		btnAssign := markup.Data("📋 Назначить", "relay", "assign", strconv.Itoa(r.ID))
		rows = append(rows, markup.Row(btnCard))
		rows = append(rows, markup.Row(btnClients, btnAssign))
	}

	btnAdd := markup.Data("➕ Добавить релей", "relay", "add")
	btnMain := markup.Data("🏠 В меню", "menu", "main")
	rows = append(rows, markup.Row(btnAdd, btnMain))

	markup.Inline(rows...)

	if edit {
		return EditOrSend(c, sb.String(), markup)
	}
	return c.Send(sb.String(), markup, tele.ModeMarkdown)
}

// ShowRelayCard отображает подробную карточку конкретного релея.
func (m *Manager) ShowRelayCard(c tele.Context, relayID int, edit bool) error {
	m.fsm.Clear(c.Sender().ID)

	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище релеев не доступно.")
	}

	r, ok := rs.GetByID(relayID)
	if !ok {
		RespondAlert(c, "Релей не найден")
		return m.ShowRelaysMenu(c, edit)
	}

	allClients := m.bot.ClientsStorage().List()
	cnt := relays.CountActiveOnRelay(allClients, r.ID)
	left := r.MaxClients - cnt
	if left < 0 {
		left = 0
	}

	statusLabel := "✅ активен"
	if !r.Enabled {
		statusLabel = "⛔ отключён"
	}

	text := fmt.Sprintf(
		"📡 *%s*\n"+
			"🆔 ID: `%d`\n"+
			"🌐 Host: `%s`\n"+
			"🔌 VLESS: `%d`  |  XHTTP: `%d`\n"+
			"👥 Клиентов: *%d* / %d (осталось %d мест)\n"+
			"📊 Статус: %s",
		r.Name,
		r.ID,
		r.Host,
		r.PortVLESS,
		r.PortXHTTP,
		cnt,
		r.MaxClients,
		left,
		statusLabel,
	)

	markup := &tele.ReplyMarkup{}
	btnLimit := markup.Data("✏️ Изменить лимит", "relay", "edit_limit", strconv.Itoa(r.ID))
	btnHost := markup.Data("📡 Изменить host", "relay", "edit_host", strconv.Itoa(r.ID))

	var btnToggle tele.Btn
	if r.Enabled {
		btnToggle = markup.Data("⛔ Отключить", "relay", "toggle", strconv.Itoa(r.ID))
	} else {
		btnToggle = markup.Data("▶️ Включить", "relay", "toggle", strconv.Itoa(r.ID))
	}

	btnClients := markup.Data(fmt.Sprintf("👥 Список клиентов (%d)", cnt), "relay", "clients", strconv.Itoa(r.ID))
	btnAssign := markup.Data("📋 Назначить клиента", "relay", "assign", strconv.Itoa(r.ID))

	var btnDelete tele.Btn
	if cnt == 0 {
		btnDelete = markup.Data("🗑 Удалить релей", "relay", "delete", strconv.Itoa(r.ID))
	}

	btnBackRelays := markup.Data("⬅️ К релеям", "menu", "relays")
	btnBackMain := markup.Data("🏠 В меню", "menu", "main")

	var rows []tele.Row
	rows = append(rows, markup.Row(btnLimit, btnHost))
	rows = append(rows, markup.Row(btnToggle, btnAssign))
	rows = append(rows, markup.Row(btnClients))
	if btnDelete.Text != "" {
		rows = append(rows, markup.Row(btnDelete))
	}
	rows = append(rows, markup.Row(btnBackRelays, btnBackMain))

	markup.Inline(rows...)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowRelayClients отображает список клиентов, подключенных к данному релею.
func (m *Manager) ShowRelayClients(c tele.Context, relayID int, edit bool) error {
	allClients := m.bot.ClientsStorage().List()

	nodeName := "Direct NL"
	if relayID > 0 {
		if rs := m.bot.RelaysStorage(); rs != nil {
			if r, ok := rs.GetByID(relayID); ok {
				nodeName = r.Name
			}
		}
	}

	var matched []*bot.Client
	for _, cl := range allClients {
		if cl.RelayID == relayID {
			matched = append(matched, cl)
		}
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	if len(matched) == 0 {
		rows = append(rows, markup.Row(markup.Data("На этом узле нет клиентов", "noop")))
	} else {
		for _, cl := range matched {
			daysLeftStr := bot.FormatDaysLeft(cl.ExpiresAt)
			label := fmt.Sprintf("%s (%s)", cl.Name, daysLeftStr)
			if cl.Revoked {
				label = "🔴 " + label
			} else {
				label = "🟢 " + label
			}
			btn := markup.Data(label, "client", "view", cl.UUID)
			rows = append(rows, markup.Row(btn))
		}
	}

	btnBack := markup.Data("⬅️ К релею", "relay", "view", strconv.Itoa(relayID))
	if relayID == 0 {
		btnBack = markup.Data("⬅️ К релеям", "menu", "relays")
	}
	btnMain := markup.Data("🏠 В меню", "menu", "main")
	rows = append(rows, markup.Row(btnBack, btnMain))

	markup.Inline(rows...)

	text := fmt.Sprintf("👥 *Клиенты на узле %s*\nВсего привязано: *%d*", nodeName, len(matched))
	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowChooseRelay выводит меню выбора релея для клиента.
func (m *Manager) ShowChooseRelay(c tele.Context, clientUUID string, edit bool) error {
	client, ok := m.bot.ClientsStorage().Get(clientUUID)
	if !ok {
		RespondAlert(c, "Клиент не найден")
		return m.ShowClientsList(c, 1, "", edit)
	}

	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище релеев не доступно.")
	}

	allClients := m.bot.ClientsStorage().List()
	list := rs.GetAll()

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Direct NL
	btnDirectLabel := "🌐 Direct NL (без лимита)"
	if client.RelayID == 0 {
		btnDirectLabel = "✅ 🌐 Direct NL (текущий)"
	}
	btnDirect := markup.Data(btnDirectLabel, "client", "set_relay", client.UUID, "0")
	rows = append(rows, markup.Row(btnDirect))

	for _, r := range list {
		cnt := relays.CountActiveOnRelay(allClients, r.ID)
		label := fmt.Sprintf("📡 %s (%d/%d)", r.Name, cnt, r.MaxClients)
		if client.RelayID == r.ID {
			label = "✅ " + label + " (текущий)"
		} else if !r.Enabled {
			label = "⛔ " + label + " (отключён)"
		} else if cnt >= r.MaxClients {
			label = "⚠️ " + label + " (заполнен)"
		}

		btn := markup.Data(label, "client", "set_relay", client.UUID, strconv.Itoa(r.ID))
		rows = append(rows, markup.Row(btn))
	}

	btnCancel := markup.Data("⬅️ Отмена", "client", "view", client.UUID)
	rows = append(rows, markup.Row(btnCancel))

	markup.Inline(rows...)

	text := fmt.Sprintf("🔄 *Переключение узла для клиента «%s»*\nВыберите целевой релей:", client.Name)
	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowAssignClientToRelay выводит список клиентов для назначения на конкретный релей.
func (m *Manager) ShowAssignClientToRelay(c tele.Context, relayID int, edit bool) error {
	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище релеев не доступно.")
	}

	r, ok := rs.GetByID(relayID)
	if !ok {
		RespondAlert(c, "Релей не найден")
		return m.ShowRelaysMenu(c, edit)
	}

	allClients := m.bot.ClientsStorage().List()
	cnt := relays.CountActiveOnRelay(allClients, r.ID)
	if cnt >= r.MaxClients {
		RespondAlert(c, fmt.Sprintf("Релей заполнен (%d/%d). Увеличьте лимит.", cnt, r.MaxClients))
		return m.ShowRelayCard(c, relayID, edit)
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	count := 0
	for _, cl := range allClients {
		if cl.RelayID != relayID && cl.IsActive() {
			btn := markup.Data(fmt.Sprintf("👤 %s", cl.Name), "client", "set_relay", cl.UUID, strconv.Itoa(relayID))
			rows = append(rows, markup.Row(btn))
			count++
			if count >= 30 {
				break
			}
		}
	}

	if count == 0 {
		rows = append(rows, markup.Row(markup.Data("Все активные клиенты уже на этом узле", "noop")))
	}

	btnBack := markup.Data("⬅️ К релею", "relay", "view", strconv.Itoa(relayID))
	rows = append(rows, markup.Row(btnBack))

	markup.Inline(rows...)

	text := fmt.Sprintf("📋 *Назначение клиента на %s*\n(Занято %d/%d). Выберите клиента:", r.Name, cnt, r.MaxClients)
	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// HandleSetRelay назначает клиента на указанный релей с полной проверкой лимитов.
func (m *Manager) HandleSetRelay(c tele.Context, clientUUID string, targetRelayID int) error {
	client, ok := m.bot.ClientsStorage().Get(clientUUID)
	if !ok {
		RespondAlert(c, "Клиент не найден")
		return nil
	}

	var nodeName string
	if targetRelayID == 0 {
		nodeName = "Direct NL (Нидерланды)"
	} else {
		rs := m.bot.RelaysStorage()
		if rs == nil {
			RespondAlert(c, "Хранилище релеев не доступно.")
			return nil
		}
		r, exists := rs.GetByID(targetRelayID)
		if !exists {
			RespondAlert(c, "Релей не найден")
			return nil
		}
		if !r.Enabled {
			RespondAlert(c, fmt.Sprintf("Релей «%s» отключён. Сначала включите его в разделе Релеи.", r.Name))
			return nil
		}

		allClients := m.bot.ClientsStorage().List()
		cnt := relays.CountActiveOnRelay(allClients, targetRelayID)
		if client.RelayID != targetRelayID && cnt >= r.MaxClients {
			RespondAlert(c, fmt.Sprintf("Релей %s заполнен (%d/%d). Выберите другой узел или увеличьте лимит.", r.Name, cnt, r.MaxClients))
			return nil
		}
		nodeName = r.Name
	}

	updated, err := m.bot.ClientsStorage().SetRelayID(clientUUID, targetRelayID)
	if err != nil {
		RespondAlert(c, "Ошибка: "+err.Error())
		return nil
	}

	m.LogRelayChange(c.Sender().ID, "assign", targetRelayID, fmt.Sprintf("client=%s to relay=%d", clientUUID[:8], targetRelayID))
	m.bot.LogAdminAction(c.Sender().ID, "set_relay", clientUUID, true)

	// Уведомление клиенту в Telegram, если он привязан к боту
	if user, found := m.bot.Storage().FindByUUID(clientUUID); found && user.TelegramID != 0 {
		notifyText := fmt.Sprintf(
			"📡 *Ваш узел подключения изменён!*\n\n"+
				"Новый узел: *%s*\n"+
				"Пожалуйста, переподключитесь к VPN, чтобы применить новые параметры маршрутизации.",
			nodeName,
		)
		_, _ = m.bot.TeleBot().Send(&tele.User{ID: user.TelegramID}, notifyText, tele.ModeMarkdown)
	}

	_ = c.Send(fmt.Sprintf("✅ Клиент *%s* переключён на: *%s*", updated.Name, nodeName), tele.ModeMarkdown)
	return m.ShowClientCard(c, updated, false)
}

// StartAddRelayWizard запускает FSM добавления нового релея.
func (m *Manager) StartAddRelayWizard(c tele.Context, edit bool) error {
	m.fsm.Set(c.Sender().ID, "relay_add_name", "", 0, make(map[string]string))

	text := "➕ *Добавление нового релея*\n\n" +
		"Шаг 1 из 5: Введите название релея\n" +
		"_(например: `Relay 2 (СПб, RU)`):_"

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "menu", "relays")
	markup.Inline(markup.Row(btnCancel))

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// HandleFSMAddRelayName обрабатывает ввод имени релея.
func (m *Manager) HandleFSMAddRelayName(c tele.Context, state *State, text string) error {
	name := strings.TrimSpace(text)
	if name == "" || len([]rune(name)) > 64 {
		return c.Send("Название должно содержать от 1 до 64 символов. Попробуйте снова:")
	}

	state.Data["name"] = name
	state.Action = "relay_add_host"
	m.fsm.Set(c.Sender().ID, state.Action, "", 0, state.Data)

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "menu", "relays")
	markup.Inline(markup.Row(btnCancel))

	msg := fmt.Sprintf("Шаг 2 из 5: Название: *%s*\n\nВведите host (IPv4 адрес или домен, например `relay2.domain.com`):", name)
	return c.Send(msg, markup, tele.ModeMarkdown)
}

// HandleFSMAddRelayHost обрабатывает ввод хоста релея.
func (m *Manager) HandleFSMAddRelayHost(c tele.Context, state *State, text string) error {
	host := strings.TrimSpace(text)
	if strings.ContainsAny(host, " \t\r\n") || host == "" {
		return c.Send("Host не должен содержать пробелов. Введите валидный IPv4 адрес или доменное имя:")
	}
	if ip := net.ParseIP(host); ip == nil {
		if !strings.Contains(host, ".") {
			return c.Send("Неверный формат хоста. Введите валидный IPv4 адрес или доменное имя:")
		}
	}

	state.Data["host"] = host
	state.Action = "relay_add_port_vless"
	m.fsm.Set(c.Sender().ID, state.Action, "", 0, state.Data)

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "menu", "relays")
	markup.Inline(markup.Row(btnCancel))

	msg := "Шаг 3 из 5: Введите порт VLESS\n_(по умолчанию `443`, отправьте `-` или `443`):_"
	return c.Send(msg, markup, tele.ModeMarkdown)
}

// HandleFSMAddRelayPortVLESS обрабатывает ввод порта VLESS.
func (m *Manager) HandleFSMAddRelayPortVLESS(c tele.Context, state *State, text string) error {
	text = strings.TrimSpace(text)
	port := 443
	if text != "-" && text != "" {
		p, err := strconv.Atoi(text)
		if err != nil || p < 1 || p > 65535 {
			return c.Send("Порт должен быть числом от 1 до 65535. Попробуйте снова:")
		}
		port = p
	}

	state.Data["port_vless"] = strconv.Itoa(port)
	state.Action = "relay_add_port_xhttp"
	m.fsm.Set(c.Sender().ID, state.Action, "", 0, state.Data)

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "menu", "relays")
	markup.Inline(markup.Row(btnCancel))

	msg := "Шаг 4 из 5: Введите порт XHTTP\n_(по умолчанию `8443`, отправьте `-` или `8443`):_"
	return c.Send(msg, markup, tele.ModeMarkdown)
}

// HandleFSMAddRelayPortXHTTP обрабатывает ввод порта XHTTP.
func (m *Manager) HandleFSMAddRelayPortXHTTP(c tele.Context, state *State, text string) error {
	text = strings.TrimSpace(text)
	port := 8443
	if text != "-" && text != "" {
		p, err := strconv.Atoi(text)
		if err != nil || p < 1 || p > 65535 {
			return c.Send("Порт должен быть числом от 1 до 65535. Попробуйте снова:")
		}
		port = p
	}

	state.Data["port_xhttp"] = strconv.Itoa(port)
	state.Action = "relay_add_max_clients"
	m.fsm.Set(c.Sender().ID, state.Action, "", 0, state.Data)

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "menu", "relays")
	markup.Inline(markup.Row(btnCancel))

	msg := "Шаг 5 из 5: Введите максимальный лимит клиентов\n_(по умолчанию `10`, отправьте `-` или число от 1 до 1000):_"
	return c.Send(msg, markup, tele.ModeMarkdown)
}

// HandleFSMAddRelayMaxClients обрабатывает ввод лимита и показывает экран подтверждения.
func (m *Manager) HandleFSMAddRelayMaxClients(c tele.Context, state *State, text string) error {
	text = strings.TrimSpace(text)
	limit := 10
	if text != "-" && text != "" {
		l, err := strconv.Atoi(text)
		if err != nil || l < 1 || l > 1000 {
			return c.Send("Лимит клиентов должен быть числом от 1 до 1000. Попробуйте снова:")
		}
		limit = l
	}

	state.Data["max_clients"] = strconv.Itoa(limit)
	state.Action = "relay_add_confirm"
	m.fsm.Set(c.Sender().ID, state.Action, "", 0, state.Data)

	card := fmt.Sprintf(
		"📋 *Подтверждение создания релея*\n\n"+
			"📡 Название: *%s*\n"+
			"🌐 Host: `%s`\n"+
			"🔌 Порт VLESS: `%s`\n"+
			"🔌 Порт XHTTP: `%s`\n"+
			"👥 Лимит клиентов: *%d*\n\n"+
			"Создать узел?",
		state.Data["name"],
		state.Data["host"],
		state.Data["port_vless"],
		state.Data["port_xhttp"],
		limit,
	)

	markup := &tele.ReplyMarkup{}
	btnConfirm := markup.Data("✅ Создать релей", "relay", "add_confirm")
	btnCancel := markup.Data("✖️ Отмена", "menu", "relays")
	markup.Inline(markup.Row(btnConfirm, btnCancel))

	return c.Send(card, markup, tele.ModeMarkdown)
}

// HandleConfirmAddRelay создаёт релей в хранилище.
func (m *Manager) HandleConfirmAddRelay(c tele.Context) error {
	state, ok := m.fsm.Get(c.Sender().ID)
	if !ok || state.Action != "relay_add_confirm" {
		RespondAlert(c, "Сессия истекла")
		return m.ShowRelaysMenu(c, true)
	}

	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище релеев не доступно.")
	}

	portVLESS, _ := strconv.Atoi(state.Data["port_vless"])
	portXHTTP, _ := strconv.Atoi(state.Data["port_xhttp"])
	maxClients, _ := strconv.Atoi(state.Data["max_clients"])

	newRelay := relays.Relay{
		ID:         rs.NextID(),
		Name:       state.Data["name"],
		Host:       state.Data["host"],
		PortVLESS:  portVLESS,
		PortXHTTP:  portXHTTP,
		MaxClients: maxClients,
		Enabled:    true,
	}

	added, err := rs.Add(newRelay)
	if err != nil {
		m.fsm.Clear(c.Sender().ID)
		RespondAlert(c, "Ошибка: "+err.Error())
		return m.ShowRelaysMenu(c, true)
	}

	m.fsm.Clear(c.Sender().ID)
	m.LogRelayChange(c.Sender().ID, "add", added.ID, fmt.Sprintf("created %s host=%s", added.Name, added.Host))
	_ = c.Send(fmt.Sprintf("✅ Релей *%s* (ID `%d`) успешно создан!", added.Name, added.ID), tele.ModeMarkdown)
	return m.ShowRelayCard(c, added.ID, false)
}

// StartEditRelayLimit запускает FSM изменения лимита.
func (m *Manager) StartEditRelayLimit(c tele.Context, relayID int) error {
	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище релеев не доступно.")
	}
	r, ok := rs.GetByID(relayID)
	if !ok {
		RespondAlert(c, "Релей не найден")
		return m.ShowRelaysMenu(c, true)
	}

	data := map[string]string{"relay_id": strconv.Itoa(relayID)}
	m.fsm.Set(c.Sender().ID, "relay_edit_limit", "", 0, data)

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "relay", "view", strconv.Itoa(relayID))
	markup.Inline(markup.Row(btnCancel))

	text := fmt.Sprintf("✏️ *Изменение лимита клиентов для %s*\nТекущий лимит: *%d*\n\nВведите новый лимит (от 1 до 1000):", r.Name, r.MaxClients)
	return EditOrSend(c, text, markup)
}

// HandleFSMEditRelayLimit сохраняет обновлённый лимит.
func (m *Manager) HandleFSMEditRelayLimit(c tele.Context, state *State, text string) error {
	limit, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || limit < 1 || limit > 1000 {
		return c.Send("Лимит должен быть числом от 1 до 1000. Попробуйте снова:")
	}

	relayID, _ := strconv.Atoi(state.Data["relay_id"])
	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище не доступно.")
	}
	r, ok := rs.GetByID(relayID)
	if !ok {
		return c.Send("Релей не найден.")
	}

	r.MaxClients = limit
	if err := rs.Update(*r); err != nil {
		return c.Send("Ошибка обновления: " + err.Error())
	}

	m.fsm.Clear(c.Sender().ID)
	m.LogRelayChange(c.Sender().ID, "config", relayID, fmt.Sprintf("max_clients=%d", limit))
	_ = c.Send(fmt.Sprintf("✅ Лимит для *%s* изменён на *%d* клиентов.", r.Name, limit), tele.ModeMarkdown)
	return m.ShowRelayCard(c, relayID, false)
}

// StartEditRelayHost запускает FSM изменения хоста релея.
func (m *Manager) StartEditRelayHost(c tele.Context, relayID int) error {
	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище релеев не доступно.")
	}
	r, ok := rs.GetByID(relayID)
	if !ok {
		RespondAlert(c, "Релей не найден")
		return m.ShowRelaysMenu(c, true)
	}

	data := map[string]string{"relay_id": strconv.Itoa(relayID)}
	m.fsm.Set(c.Sender().ID, "relay_edit_host", "", 0, data)

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "relay", "view", strconv.Itoa(relayID))
	markup.Inline(markup.Row(btnCancel))

	text := fmt.Sprintf(
		"📡 *Изменение хоста релея %s*\n"+
			"Текущий адрес: `%s`\n\n"+
			"⚠️ *Внимание:* клиенты на этом релее переподключатся на новый адрес при следующем запросе параметров.\n\n"+
			"Введите новый host (IPv4 или домен):",
		r.Name,
		r.Host,
	)
	return EditOrSend(c, text, markup)
}

// HandleFSMEditRelayHost сохраняет обновлённый хост.
func (m *Manager) HandleFSMEditRelayHost(c tele.Context, state *State, text string) error {
	host := strings.TrimSpace(text)
	if strings.ContainsAny(host, " \t\r\n") || host == "" {
		return c.Send("Host не должен содержать пробелов. Введите валидный IPv4 адрес или домен:")
	}

	relayID, _ := strconv.Atoi(state.Data["relay_id"])
	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище не доступно.")
	}
	r, ok := rs.GetByID(relayID)
	if !ok {
		return c.Send("Релей не найден.")
	}

	r.Host = host
	if err := rs.Update(*r); err != nil {
		return c.Send("Ошибка обновления: " + err.Error())
	}

	m.fsm.Clear(c.Sender().ID)
	m.LogRelayChange(c.Sender().ID, "config", relayID, fmt.Sprintf("host=%s", host))
	_ = c.Send(fmt.Sprintf("✅ Адрес хоста для *%s* изменён на `%s`.", r.Name, host), tele.ModeMarkdown)
	return m.ShowRelayCard(c, relayID, false)
}

// HandleToggleRelay переключает активность релея (с проверкой привязанных клиентов).
func (m *Manager) HandleToggleRelay(c tele.Context, relayID int) error {
	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище релеев не доступно.")
	}
	r, ok := rs.GetByID(relayID)
	if !ok {
		RespondAlert(c, "Релей не найден")
		return m.ShowRelaysMenu(c, true)
	}

	allClients := m.bot.ClientsStorage().List()
	cnt := relays.CountActiveOnRelay(allClients, r.ID)

	// Если мы отключаем релей и на нём есть активные клиенты — требуем подтверждения
	if r.Enabled && cnt > 0 {
		markup := &tele.ReplyMarkup{}
		btnConfirm := markup.Data("✅ Да, перевести на Direct NL", "relay", "toggle_confirm", strconv.Itoa(relayID))
		btnCancel := markup.Data("✖️ Отмена", "relay", "view", strconv.Itoa(relayID))
		markup.Inline(markup.Row(btnConfirm), markup.Row(btnCancel))

		text := fmt.Sprintf(
			"⚠️ *Предупреждение при отключении релея*\n\n"+
				"На релее *%s* сейчас привязано *%d* клиентов.\n\n"+
				"При отключении все они будут автоматически переведены на *Direct NL (Нидерланды)*.\n\n"+
				"Продолжить отключение?",
			r.Name,
			cnt,
		)
		return EditOrSend(c, text, markup)
	}

	// Иначе переключаем сразу
	updated, err := rs.Toggle(relayID)
	if err != nil {
		RespondAlert(c, "Ошибка: "+err.Error())
		return nil
	}

	m.LogRelayChange(c.Sender().ID, "toggle", relayID, fmt.Sprintf("enabled=%v", updated.Enabled))
	return m.ShowRelayCard(c, relayID, true)
}

// HandleToggleConfirm подтверждает отключение релея и переводит всех клиентов на Direct NL.
func (m *Manager) HandleToggleConfirm(c tele.Context, relayID int) error {
	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище не доступно.")
	}
	r, ok := rs.GetByID(relayID)
	if !ok {
		RespondAlert(c, "Релей не найден")
		return m.ShowRelaysMenu(c, true)
	}

	allClients := m.bot.ClientsStorage().List()
	transferred := 0
	for _, cl := range allClients {
		if cl.RelayID == relayID {
			_, _ = m.bot.ClientsStorage().SetRelayID(cl.UUID, 0)
			transferred++

			// Уведомление клиенту
			if user, found := m.bot.Storage().FindByUUID(cl.UUID); found && user.TelegramID != 0 {
				notifyText := fmt.Sprintf(
					"📡 *Узел %s временно отключён на обслуживание.*\n"+
						"Ваше подключение переведено на *Direct NL (Нидерланды)*.\n"+
						"Пожалуйста, переподключитесь к VPN.",
					r.Name,
				)
				_, _ = m.bot.TeleBot().Send(&tele.User{ID: user.TelegramID}, notifyText, tele.ModeMarkdown)
			}
		}
	}

	r.Enabled = false
	_ = rs.Update(*r)

	m.LogRelayChange(c.Sender().ID, "toggle", relayID, fmt.Sprintf("disabled with %d clients transferred to direct NL", transferred))
	_ = c.Send(fmt.Sprintf("⛔ Релей *%s* отключён. Переведено клиентов на Direct NL: *%d*", r.Name, transferred), tele.ModeMarkdown)
	return m.ShowRelayCard(c, relayID, false)
}

// HandleDeleteRelay удаляет релей, если на нём 0 клиентов.
func (m *Manager) HandleDeleteRelay(c tele.Context, relayID int) error {
	rs := m.bot.RelaysStorage()
	if rs == nil {
		return c.Send("Хранилище не доступно.")
	}

	allClients := m.bot.ClientsStorage().List()
	cnt := relays.CountActiveOnRelay(allClients, relayID)
	if cnt > 0 {
		RespondAlert(c, fmt.Sprintf("Нельзя удалить релей с клиентами (%d привязано). Сначала переведите их.", cnt))
		return nil
	}

	if err := rs.Delete(relayID); err != nil {
		RespondAlert(c, "Ошибка удаления: "+err.Error())
		return nil
	}

	m.LogRelayChange(c.Sender().ID, "delete", relayID, fmt.Sprintf("deleted relay id=%d", relayID))
	RespondAlert(c, "Релей удалён")
	return m.ShowRelaysMenu(c, true)
}

// LogRelayChange логирует действия над релеями в JSON-формате.
func (m *Manager) LogRelayChange(adminID int64, action string, relayID int, details string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"event":    "relay_change",
		"admin_id": adminID,
		"action":   action,
		"relay_id": relayID,
		"details":  details,
	})
	log.Println(string(payload))
}
