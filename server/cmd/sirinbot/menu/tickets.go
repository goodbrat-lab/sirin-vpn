package menu

import (
	"fmt"
	"strconv"
	"strings"

	"sirin/server/internal/bot/tickets"
	tele "gopkg.in/telebot.v3"
)

// ShowUserTicketsList показывает список обращений текущего пользователя.
func (m *Manager) ShowUserTicketsList(c tele.Context, edit bool) error {
	userID := c.Sender().ID
	ts := m.bot.TicketsStorage()
	if ts == nil {
		RespondAlert(c, "Система тикетов временно недоступна")
		return m.ShowClientHome(c, edit)
	}

	list := ts.GetByUser(userID)
	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	if len(list) == 0 {
		text := "📋 *Мои обращения в поддержку*\n\n" +
			"У вас пока нет созданных тикетов.\n\n" +
			"Если у вас возникли сложности с подключением, воспользуйтесь разделом *«🩺 Проверить соединение»* — при обнаружении проблемы вы сможете отправить тикет в один клик с полным отчётом диагностики."

		btnDiag := markup.Data("🩺 Проверить соединение", "my", "diag")
		btnBack := markup.Data("◀ В меню", "my", "home")
		markup.Inline(markup.Row(btnDiag), markup.Row(btnBack))

		if edit {
			return EditOrSend(c, text, markup)
		}
		return c.Send(text, markup, tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 *Мои обращения в поддержку (%d):*\n\n", len(list)))

	for _, t := range list {
		statusEmoji := "🟡"
		statusText := "открыт"
		if t.Status == tickets.StatusInProgress {
			statusEmoji = "🔵"
			statusText = "в работе"
		} else if t.Status == tickets.StatusClosed {
			statusEmoji = "⚪"
			statusText = "закрыт"
		}

		devLabel := t.DeviceLabel
		if devLabel == "" {
			devLabel = "Устройство"
		}

		updatedStr := t.UpdatedAt.Format("02.01 15:04")
		sb.WriteString(fmt.Sprintf("%s *#%d* · %s (%s) — _%s_\nОбновлён: %s\n\n",
			statusEmoji, t.ID, devLabel, formatCategory(t.Category), statusText, updatedStr))

		btnView := markup.Data(fmt.Sprintf("🔍 #%d: %s", t.ID, devLabel), "ticket", "user_view", strconv.Itoa(t.ID))
		rows = append(rows, markup.Row(btnView))
	}

	btnDiag := markup.Data("🩺 Проверить соединение", "my", "diag")
	btnBack := markup.Data("◀ В меню", "my", "home")
	rows = append(rows, markup.Row(btnDiag), markup.Row(btnBack))
	markup.Inline(rows...)

	if edit {
		return EditOrSend(c, sb.String(), markup)
	}
	return c.Send(sb.String(), markup, tele.ModeMarkdown)
}

// ShowUserTicketCard показывает карточку конкретного обращения клиенту.
func (m *Manager) ShowUserTicketCard(c tele.Context, ticketID int, edit bool) error {
	ts := m.bot.TicketsStorage()
	if ts == nil {
		return nil
	}

	t, ok := ts.GetByID(ticketID)
	if !ok || t == nil || t.TelegramID != c.Sender().ID {
		RespondAlert(c, "Тикет не найден")
		return m.ShowUserTicketsList(c, edit)
	}

	statusEmoji := "🟡"
	statusText := "Открыт"
	if t.Status == tickets.StatusInProgress {
		statusEmoji = "🔵"
		statusText = "В работе"
	} else if t.Status == tickets.StatusClosed {
		statusEmoji = "⚪"
		statusText = "Закрыт"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🎫 *Обращение #%d*\n\n", t.ID))
	sb.WriteString(fmt.Sprintf("Статус: %s *%s*\n", statusEmoji, statusText))
	sb.WriteString(fmt.Sprintf("Категория: *%s*\n", formatCategory(t.Category)))
	if t.DeviceLabel != "" {
		sb.WriteString(fmt.Sprintf("Устройство: *%s* (%s)\n", t.DeviceLabel, formatPlatform(t.DevicePlatform)))
	}
	sb.WriteString(fmt.Sprintf("Создан: %s\n\n", t.CreatedAt.Format("02.01 15:04")))

	if len(t.Diagnostics) > 0 {
		sb.WriteString("*Диагностика:* ")
		var diagParts []string
		for _, d := range t.Diagnostics {
			dIcon := "✅"
			if d.Status == "warn" {
				dIcon = "⚠️"
			} else if d.Status == "fail" {
				dIcon = "❌"
			}
			diagParts = append(diagParts, fmt.Sprintf("%s %s", dIcon, d.Name))
		}
		sb.WriteString(strings.Join(diagParts, " · ") + "\n\n")
	}

	sb.WriteString("*Переписка:*\n")
	if len(t.Messages) == 0 {
		sb.WriteString("_(нет сообщений)_\n")
	} else {
		for _, msg := range t.Messages {
			fromLabel := "👤 Вы"
			if msg.From == string(tickets.SenderAdmin) {
				fromLabel = "🛡 Поддержка"
			}
			sb.WriteString(fmt.Sprintf("*[%s %s]:*\n%s\n\n", fromLabel, msg.TS.Format("15:04"), msg.Text))
		}
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	if t.Status != tickets.StatusClosed {
		btnReply := markup.Data("💬 Ответить", "ticket", "user_reply", strconv.Itoa(t.ID))
		btnClose := markup.Data("🔒 Закрыть обращение", "ticket", "user_close", strconv.Itoa(t.ID))
		rows = append(rows, markup.Row(btnReply, btnClose))
	}

	btnBack := markup.Data("◀ К списку обращений", "ticket", "user_list")
	rows = append(rows, markup.Row(btnBack))
	markup.Inline(rows...)

	if edit {
		return EditOrSend(c, sb.String(), markup)
	}
	return c.Send(sb.String(), markup, tele.ModeMarkdown)
}

// ShowAdminTicketsList показывает список тикетов администратору.
func (m *Manager) ShowAdminTicketsList(c tele.Context, openOnly bool, edit bool) error {
	if !m.bot.IsAdmin(c.Sender().ID) {
		RespondAlert(c, "Доступ запрещён")
		return nil
	}

	ts := m.bot.TicketsStorage()
	if ts == nil {
		RespondAlert(c, "Хранилище тикетов не инициализировано")
		return nil
	}

	var list []tickets.Ticket
	if openOnly {
		list = ts.GetOpen()
	} else {
		list = ts.GetAll()
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	title := "🆘 *Тикеты технической поддержки*"
	if openOnly {
		title += " (только открытые)"
	} else {
		title += " (все)"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\nВсего в списке: *%d*\n\n", title, len(list)))

	if len(list) == 0 {
		sb.WriteString("Тикетов нет.")
	} else {
		for _, t := range list {
			statusEmoji := "🟡"
			if t.Status == tickets.StatusInProgress {
				statusEmoji = "🔵"
			} else if t.Status == tickets.StatusClosed {
				statusEmoji = "⚪"
			}

			userHandle := t.Username
			if userHandle == "" {
				userHandle = fmt.Sprintf("id:%d", t.TelegramID)
			}
			devInfo := t.DeviceLabel
			if devInfo == "" {
				devInfo = t.DevicePlatform
			}

			sb.WriteString(fmt.Sprintf("%s *#%d* · %s · %s (%s)\nОбновлён: %s · Сообщений: %d\n\n",
				statusEmoji, t.ID, userHandle, devInfo, formatCategory(t.Category), t.UpdatedAt.Format("02.01 15:04"), len(t.Messages)))

			btnCard := markup.Data(fmt.Sprintf("🔍 #%d (%s)", t.ID, userHandle), "ticket", "adm_view", strconv.Itoa(t.ID))
			rows = append(rows, markup.Row(btnCard))
		}
	}

	// Фильтр
	var btnToggleFilter tele.Btn
	if openOnly {
		btnToggleFilter = markup.Data("📋 Показать все тикеты", "ticket", "adm_all")
	} else {
		btnToggleFilter = markup.Data("🟡 Только открытые", "ticket", "adm_open")
	}

	btnBack := markup.Data("⬅️ В главное меню", "menu", "main")
	rows = append(rows, markup.Row(btnToggleFilter), markup.Row(btnBack))
	markup.Inline(rows...)

	if edit {
		return EditOrSend(c, sb.String(), markup)
	}
	return c.Send(sb.String(), markup, tele.ModeMarkdown)
}

// ShowAdminTicketCard отображает карточку тикета для администратора.
func (m *Manager) ShowAdminTicketCard(c tele.Context, ticketID int, edit bool) error {
	if !m.bot.IsAdmin(c.Sender().ID) {
		return nil
	}

	ts := m.bot.TicketsStorage()
	if ts == nil {
		return nil
	}

	t, ok := ts.GetByID(ticketID)
	if !ok || t == nil {
		RespondAlert(c, "Тикет не найден")
		return m.ShowAdminTicketsList(c, true, edit)
	}

	statusEmoji := "🟡"
	statusText := "open"
	if t.Status == tickets.StatusInProgress {
		statusEmoji = "🔵"
		statusText = "in_progress"
	} else if t.Status == tickets.StatusClosed {
		statusEmoji = "⚪"
		statusText = "closed"
	}

	userHandle := t.Username
	if userHandle == "" {
		userHandle = fmt.Sprintf("id:%d", t.TelegramID)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🆘 *Тикет #%d*\n\n", t.ID))
	sb.WriteString(fmt.Sprintf("👤 Клиент: %s (`%d`)\n", userHandle, t.TelegramID))
	sb.WriteString(fmt.Sprintf("📱 Устройство: *%s* (%s, v%s)\n", t.DeviceLabel, t.DevicePlatform, t.DeviceAppVersion))
	if t.DeviceUUID != "" {
		sb.WriteString(fmt.Sprintf("🔑 Ключ: `%s`\n", tickets.MaskUUID(t.DeviceUUID)))
	}
	sb.WriteString(fmt.Sprintf("📂 Категория: *%s* — %s *%s*\n", formatCategory(t.Category), statusEmoji, statusText))
	sb.WriteString(fmt.Sprintf("📅 Создан: %s · Обновлён: %s\n\n", t.CreatedAt.Format("02.01 15:04"), t.UpdatedAt.Format("02.01 15:04")))

	if len(t.Diagnostics) > 0 {
		sb.WriteString("📊 *Результаты диагностики:*\n")
		for _, d := range t.Diagnostics {
			dIcon := "✅"
			if d.Status == "warn" {
				dIcon = "⚠️"
			} else if d.Status == "fail" {
				dIcon = "❌"
			}
			sb.WriteString(fmt.Sprintf("• %s *%s*: %s\n", dIcon, d.Name, d.Message))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("💬 *Переписка:*\n")
	if len(t.Messages) == 0 {
		sb.WriteString("_(нет сообщений)_\n")
	} else {
		for _, msg := range t.Messages {
			fromLabel := fmt.Sprintf("👤 %s", userHandle)
			if msg.From == string(tickets.SenderAdmin) {
				fromLabel = "🛡 Поддержка (Вы)"
			}
			sb.WriteString(fmt.Sprintf("*[%s %s]:*\n%s\n\n", fromLabel, msg.TS.Format("15:04"), msg.Text))
		}
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	btnReply := markup.Data("💬 Ответить клиенту", "ticket", "adm_reply", strconv.Itoa(t.ID))
	var btnStatus tele.Btn
	if t.Status != tickets.StatusClosed {
		btnStatus = markup.Data("🔒 Закрыть тикет", "ticket", "adm_close", strconv.Itoa(t.ID))
	} else {
		btnStatus = markup.Data("🔓 Открыть заново", "ticket", "adm_reopen", strconv.Itoa(t.ID))
	}
	rows = append(rows, markup.Row(btnReply, btnStatus))

	// Кнопка перехода в карточку клиента, если UUID найден в базе
	if t.DeviceUUID != "" {
		if _, exists := m.bot.ClientsStorage().Get(t.DeviceUUID); exists {
			btnClientCard := markup.Data("👤 Карточка клиента", "client", "view", t.DeviceUUID)
			rows = append(rows, markup.Row(btnClientCard))
		}
	}

	btnBack := markup.Data("⬅️ К списку тикетов", "ticket", "adm_open")
	rows = append(rows, markup.Row(btnBack))
	markup.Inline(rows...)

	if edit {
		return EditOrSend(c, sb.String(), markup)
	}
	return c.Send(sb.String(), markup, tele.ModeMarkdown)
}

// HandleCreateTicketFromDiag создает тикет на основе последней диагностики.
func (m *Manager) HandleCreateTicketFromDiag(c tele.Context, userText string) error {
	userID := c.Sender().ID
	ts := m.bot.TicketsStorage()
	if ts == nil {
		RespondAlert(c, "Система тикетов недоступна")
		return m.ShowClientHome(c, false)
	}

	report := GetLastDiagnosticReport(userID)
	var diagItems []tickets.DiagnosticItem
	category := "connection"
	deviceUUID := ""
	deviceLabel := "Устройство"
	devicePlatform := "android"
	deviceVersion := "unknown"

	if report != nil {
		deviceUUID = report.ClientUUID
		devicePlatform = report.Input.Platform
		deviceVersion = report.Input.AppVersion

		switch report.Input.IssueType {
		case "slow":
			category = "speed"
		case "sites_blocked":
			category = "sites"
		case "key_issue":
			category = "key"
		case "other":
			category = "other"
		default:
			category = "connection"
		}

		for _, ch := range report.Checks {
			diagItems = append(diagItems, tickets.DiagnosticItem{
				Name:    ch.Name,
				Status:  string(ch.Status),
				Message: ch.Message,
			})
		}
	}

	if uk, ok := m.bot.Storage().Get(userID); ok && uk != nil {
		if d, found := uk.GetDevice(deviceUUID); found && d != nil {
			deviceLabel = d.Label
			if devicePlatform == "" {
				devicePlatform = d.Platform
			}
			if deviceVersion == "" || deviceVersion == "unknown" {
				deviceVersion = d.AppVersion
			}
		}
	}

	username := c.Sender().Username
	if username != "" {
		username = "@" + username
	} else {
		username = c.Sender().FirstName
	}

	subject := fmt.Sprintf("Проблема с %s (%s)", formatCategory(category), deviceLabel)
	msgText := userText
	if msgText == "" {
		msgText = "Автоматический запрос из диагностики соединения."
	}

	ticket, err := ts.CreateTicket(userID, username, tickets.DeviceContext{
		UUID:       deviceUUID,
		Label:      deviceLabel,
		Platform:   devicePlatform,
		AppVersion: deviceVersion,
	}, category, subject, msgText, diagItems)

	if err != nil {
		markup := &tele.ReplyMarkup{}
		btnBack := markup.Data("◀ В меню", "my", "home")
		markup.Inline(markup.Row(btnBack))
		return c.Send(fmt.Sprintf("❌ *Не удалось создать тикет:*\n%s", err.Error()), markup, tele.ModeMarkdown)
	}

	m.fsm.Clear(userID)

	// Уведомление администраторам
	m.notifyAdminsNewTicket(ticket)

	text := fmt.Sprintf("✅ *Обращение #%d успешно создано!*\n\n"+
		"📱 Устройство: *%s*\n"+
		"📂 Категория: *%s*\n\n"+
		"Поддержка ответит вам в этом чате. Вы также можете просматривать статус в разделе *«📋 Мои обращения»*.",
		ticket.ID, ticket.DeviceLabel, formatCategory(ticket.Category))

	markup := &tele.ReplyMarkup{}
	btnView := markup.Data("🔍 Открыть обращение", "ticket", "user_view", strconv.Itoa(ticket.ID))
	btnHome := markup.Data("◀ В главное меню", "my", "home")
	markup.Inline(markup.Row(btnView), markup.Row(btnHome))

	return c.Send(text, markup, tele.ModeMarkdown)
}

func (m *Manager) notifyAdminsNewTicket(t *tickets.Ticket) {
	adminIDs := m.bot.Config().AdminIDs()
	if len(adminIDs) == 0 {
		return
	}

	text := fmt.Sprintf("🆘 *Новое обращение в поддержку #%d*\n\n"+
		"👤 Клиент: %s (`%d`)\n"+
		"📱 Устройство: *%s* (%s)\n"+
		"📂 Категория: *%s*\n"+
		"💬 Сообщение: _%s_",
		t.ID, t.Username, t.TelegramID, t.DeviceLabel, formatPlatform(t.DevicePlatform), formatCategory(t.Category), truncateMsg(t.Messages[0].Text, 150))

	markup := &tele.ReplyMarkup{}
	btnOpen := markup.Data("🔍 Открыть тикет", "ticket", "adm_view", strconv.Itoa(t.ID))
	markup.Inline(markup.Row(btnOpen))

	tb := m.bot.TeleBot()
	for _, adminID := range adminIDs {
		_, _ = tb.Send(&tele.User{ID: adminID}, text, markup, tele.ModeMarkdown)
	}
}

// HandleTicketCallback обрабатывает inline-кнопки тикетов.
func (m *Manager) HandleTicketCallback(c tele.Context, action string, args []string) error {
	RespondSafe(c)
	userID := c.Sender().ID

	switch action {
	case "send_diag":
		return m.HandleCreateTicketFromDiag(c, "")

	case "user_list", "my_list":
		return m.ShowUserTicketsList(c, true)

	case "user_view":
		if len(args) > 0 {
			id, _ := strconv.Atoi(args[0])
			return m.ShowUserTicketCard(c, id, true)
		}

	case "user_reply":
		if len(args) > 0 {
			id, _ := strconv.Atoi(args[0])
			m.fsm.Set(userID, "ticket_user_reply", strconv.Itoa(id), 0, nil)
			text := fmt.Sprintf("💬 *Ответ по тикету #%d*\n\nОтправьте ваше сообщение в этот чат:", id)
			markup := &tele.ReplyMarkup{}
			btnCancel := markup.Data("◀ Отмена", "ticket", "user_view", strconv.Itoa(id))
			markup.Inline(markup.Row(btnCancel))
			return EditOrSend(c, text, markup)
		}

	case "user_close":
		if len(args) > 0 {
			id, _ := strconv.Atoi(args[0])
			_, err := m.bot.TicketsStorage().SetStatus(id, tickets.StatusClosed)
			if err != nil {
				RespondAlert(c, "Ошибка: "+err.Error())
				return nil
			}
			RespondAlert(c, "Обращение закрыто")
			return m.ShowUserTicketCard(c, id, true)
		}

	case "adm_open":
		return m.ShowAdminTicketsList(c, true, true)

	case "adm_all":
		return m.ShowAdminTicketsList(c, false, true)

	case "adm_view":
		if len(args) > 0 {
			id, _ := strconv.Atoi(args[0])
			return m.ShowAdminTicketCard(c, id, true)
		}

	case "adm_reply":
		if len(args) > 0 {
			id, _ := strconv.Atoi(args[0])
			m.fsm.Set(userID, "ticket_admin_reply", strconv.Itoa(id), 0, nil)
			text := fmt.Sprintf("💬 *Ответ поддержки по тикету #%d*\n\nВведите текст ответа клиенту:", id)
			markup := &tele.ReplyMarkup{}
			btnCancel := markup.Data("◀ Отмена", "ticket", "adm_view", strconv.Itoa(id))
			markup.Inline(markup.Row(btnCancel))
			return EditOrSend(c, text, markup)
		}

	case "adm_close":
		if len(args) > 0 {
			id, _ := strconv.Atoi(args[0])
			t, err := m.bot.TicketsStorage().SetStatus(id, tickets.StatusClosed)
			if err != nil {
				RespondAlert(c, "Ошибка: "+err.Error())
				return nil
			}
			// Уведомление клиенту
			m.notifyUserTicketClosed(t)
			RespondAlert(c, "Тикет закрыт")
			return m.ShowAdminTicketCard(c, id, true)
		}

	case "adm_reopen":
		if len(args) > 0 {
			id, _ := strconv.Atoi(args[0])
			_, err := m.bot.TicketsStorage().SetStatus(id, tickets.StatusOpen)
			if err != nil {
				RespondAlert(c, "Ошибка: "+err.Error())
				return nil
			}
			RespondAlert(c, "Тикет открыт заново")
			return m.ShowAdminTicketCard(c, id, true)
		}
	}

	return nil
}

func (m *Manager) notifyUserTicketClosed(t *tickets.Ticket) {
	if t == nil {
		return
	}
	text := fmt.Sprintf("🔒 *Обращение #%d закрыто поддержкой.*\n\nЕсли у вас остались вопросы, вы можете открыть новое обращение в разделе «📋 Мои обращения».", t.ID)
	tb := m.bot.TeleBot()
	_, _ = tb.Send(&tele.User{ID: t.TelegramID}, text, tele.ModeMarkdown)
}

func (m *Manager) notifyUserNewReply(t *tickets.Ticket, replyText string) {
	if t == nil {
		return
	}
	text := fmt.Sprintf("💬 *Новый ответ поддержки по тикету #%d:*\n\n%s\n\n_Чтобы ответить, перейдите в обращение ниже:_", t.ID, replyText)
	markup := &tele.ReplyMarkup{}
	btnView := markup.Data("🔍 Открыть тикет", "ticket", "user_view", strconv.Itoa(t.ID))
	markup.Inline(markup.Row(btnView))

	tb := m.bot.TeleBot()
	_, _ = tb.Send(&tele.User{ID: t.TelegramID}, text, markup, tele.ModeMarkdown)
}

func formatCategory(c string) string {
	switch c {
	case "connection":
		return "Связь и подключение"
	case "speed":
		return "Скорость и обрывы"
	case "sites":
		return "YouTube / Сайты"
	case "key":
		return "Ключ и подписка"
	case "other":
		return "Другой вопрос"
	default:
		return "Общий вопрос"
	}
}

func truncateMsg(s string, maxLen int) string {
	if len([]rune(s)) <= maxLen {
		return s
	}
	return string([]rune(s)[:maxLen]) + "..."
}
