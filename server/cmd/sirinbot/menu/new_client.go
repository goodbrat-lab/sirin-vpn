package menu

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"sirin/server/internal/bot"
	tele "gopkg.in/telebot.v3"
)

// StartNewClientWizard запускает пошаговый мастер создания клиента (Шаг 1).
func (m *Manager) StartNewClientWizard(c tele.Context, edit bool) error {
	m.fsm.Set(c.Sender().ID, "new_name", "", 0, nil)

	text := "➕ *Новый клиент — шаг 1/3*\n\n" +
		"Введите имя клиента (можно с пробелами, до 64 символов):"

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "menu", "main")
	markup.Inline(markup.Row(btnCancel))

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowNewClientStep2 отображает выбор срока действия (Шаг 2).
func (m *Manager) ShowNewClientStep2(c tele.Context, name string, edit bool) error {
	m.fsm.Update(c.Sender().ID, func(s *State) {
		s.Action = "new_days"
		s.Data["name"] = name
	})

	text := fmt.Sprintf(
		"➕ *Новый клиент — шаг 2/3*\n\n"+
			"Имя: «*%s*»\n\n"+
			"Выберите срок действия ключа:",
		name,
	)

	markup := &tele.ReplyMarkup{}
	btn7 := markup.Data("7д", "new", "days", "7")
	btn30 := markup.Data("30д", "new", "days", "30")
	btn90 := markup.Data("90д", "new", "days", "90")
	btn180 := markup.Data("180д", "new", "days", "180")
	btn365 := markup.Data("365д", "new", "days", "365")
	btnCustom := markup.Data("✏️ Другой срок", "new", "days", "custom")

	btnBack := markup.Data("⬅️ Назад", "new", "back", "1")
	btnCancel := markup.Data("✖️ Отмена", "menu", "main")

	markup.Inline(
		markup.Row(btn7, btn30, btn90),
		markup.Row(btn180, btn365),
		markup.Row(btnCustom),
		markup.Row(btnBack, btnCancel),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowNewClientCustomDays запрашивает ввод произвольного срока в днях.
func (m *Manager) ShowNewClientCustomDays(c tele.Context, name string, edit bool) error {
	m.fsm.Update(c.Sender().ID, func(s *State) {
		s.Action = "new_days_custom"
		s.Data["name"] = name
	})

	text := fmt.Sprintf(
		"➕ *Новый клиент — шаг 2/3*\n\n"+
			"Имя: «*%s*»\n\n"+
			"Введите количество дней (от 1 до 3650):",
		name,
	)

	markup := &tele.ReplyMarkup{}
	btnBack := markup.Data("⬅️ Назад", "new", "back", "2")
	btnCancel := markup.Data("✖️ Отмена", "menu", "main")
	markup.Inline(markup.Row(btnBack, btnCancel))

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowNewClientStep3 отображает экран подтверждения (Шаг 3).
func (m *Manager) ShowNewClientStep3(c tele.Context, name string, days int, edit bool) error {
	m.fsm.Update(c.Sender().ID, func(s *State) {
		s.Action = "new_confirm"
		s.Data["name"] = name
		s.Data["days"] = strconv.Itoa(days)
	})

	text := fmt.Sprintf(
		"➕ *Новый клиент — шаг 3/3*\n\n"+
			"👤 Имя: *%s*\n"+
			"⏱ Срок: *%d дней*\n\n"+
			"Будет создан ключ доступа. Подтвердить?",
		name,
		days,
	)

	markup := &tele.ReplyMarkup{}
	btnCreate := markup.Data("✅ Создать", "new", "confirm")
	btnBack := markup.Data("⬅️ Назад", "new", "back", "2")
	btnCancel := markup.Data("✖️ Отмена", "menu", "main")

	markup.Inline(
		markup.Row(btnCreate),
		markup.Row(btnBack, btnCancel),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ExecuteCreateClient создаёт клиента в хранилище и выводит карточку успеха.
func (m *Manager) ExecuteCreateClient(c tele.Context, name string, days int) error {
	return m.ExecuteCreateClientWithRelay(c, name, days, 0)
}

// ExecuteCreateClientWithRelay создаёт клиента с указанным relay_id в хранилище и выводит карточку успеха.
func (m *Manager) ExecuteCreateClientWithRelay(c tele.Context, name string, days int, relayID int) error {
	client, err := m.bot.ClientsStorage().AddWithRelay(name, days, relayID)
	if err != nil {
		RespondAlert(c, "Ошибка при создании: "+err.Error())
		return err
	}

	m.fsm.Clear(c.Sender().ID)
	m.bot.LogAdminAction(c.Sender().ID, "create_client", client.UUID, true)

	relayName := "Прямой NL (Нидерланды)"
	if client.RelayID == 1 {
		relayName = "Relay 1 (Москва, RU)"
	}

	text := fmt.Sprintf(
		"✅ *Ключ создан*\n\n"+
			"👤 Имя: *%s*\n"+
			"⏱ Срок: *%d дней*\n"+
			"📡 Узел: *%s*\n"+
			"🔑 `%s`",
		client.Name,
		days,
		relayName,
		bot.MaskUUID(client.UUID),
	)

	markup := &tele.ReplyMarkup{}
	btnShow := markup.Data("👁 Показать UUID", "client", "show_uuid", client.UUID)
	btnLink := markup.Data("📋 Ссылки", "client", "link", client.UUID)
	btnCard := markup.Data("👤 Открыть карточку", "client", "view", client.UUID)
	btnMain := markup.Data("🏠 В меню", "menu", "main")

	markup.Inline(
		markup.Row(btnShow, btnLink),
		markup.Row(btnCard, btnMain),
	)

	// Согласно требованию Шага 2.4: результат создания отправляется новым сообщением
	return c.Send(text, markup, tele.ModeMarkdown)
}

// HandleFSMNewText обрабатывает текстовые сообщения на шагах создания.
func (m *Manager) HandleFSMNewText(c tele.Context, s *State, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return c.Send("Имя не может быть пустым. Пожалуйста, введите имя:")
	}

	switch s.Action {
	case "new_name":
		if utf8.RuneCountInString(text) > bot.MaxNameLen {
			runes := []rune(text)
			text = strings.TrimSpace(string(runes[:bot.MaxNameLen]))
		}
		// Нормализация пробелов
		tokens := strings.Fields(text)
		name := strings.Join(tokens, " ")
		return m.ShowNewClientStep2(c, name, false)

	case "new_days_custom":
		days, err := strconv.Atoi(text)
		if err != nil || days <= 0 {
			return c.Send("Пожалуйста, введите целое положительное число дней (1..3650):")
		}
		if days > bot.MaxDays {
			days = bot.MaxDays
			_ = c.Send(fmt.Sprintf("⚠️ Максимальный срок ограничен 3650 днями (~10 лет). Установлено: %d дней.", days))
		}
		name := s.Data["name"]
		return m.ShowNewClientStep3(c, name, days, false)
	}

	return nil
}
