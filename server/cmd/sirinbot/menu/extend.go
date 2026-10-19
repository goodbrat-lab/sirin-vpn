package menu

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"sirin/server/internal/bot"
	tele "gopkg.in/telebot.v3"
)

// ShowExtendPrompt отображает экран продления срока.
func (m *Manager) ShowExtendPrompt(c tele.Context, uuid string, edit bool) error {
	client, ok := m.bot.ClientsStorage().Get(uuid)
	if !ok {
		RespondAlert(c, "Клиент не найден")
		return nil
	}

	daysLeftStr := bot.FormatDaysLeft(client.ExpiresAt)
	expiryStr := "бессрочно"
	if client.ExpiresAt != nil && !client.ExpiresAt.IsZero() {
		expiryStr = fmt.Sprintf("до %s (%s)", FormatDateMSK(*client.ExpiresAt), daysLeftStr)
	}

	text := fmt.Sprintf(
		"➕ *Продлить:* «*%s*»\n\n"+
			"Текущий срок: %s\n\n"+
			"На сколько дней продлить?",
		client.Name,
		expiryStr,
	)

	markup := &tele.ReplyMarkup{}
	btn7 := markup.Data("+7д", "extend", "apply", uuid, "7")
	btn30 := markup.Data("+30д", "extend", "apply", uuid, "30")
	btn90 := markup.Data("+90д", "extend", "apply", uuid, "90")
	btn365 := markup.Data("+365д", "extend", "apply", uuid, "365")

	btnCustom := markup.Data("✏️ Другой срок", "extend", "custom", uuid)
	btnBack := markup.Data("⬅️ К карточке", "client", "view", uuid)

	markup.Inline(
		markup.Row(btn7, btn30),
		markup.Row(btn90, btn365),
		markup.Row(btnCustom),
		markup.Row(btnBack),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowShortenPrompt отображает экран сокращения срока.
func (m *Manager) ShowShortenPrompt(c tele.Context, uuid string, edit bool) error {
	client, ok := m.bot.ClientsStorage().Get(uuid)
	if !ok {
		RespondAlert(c, "Клиент не найден")
		return nil
	}

	daysLeftStr := bot.FormatDaysLeft(client.ExpiresAt)
	expiryStr := "бессрочно"
	if client.ExpiresAt != nil && !client.ExpiresAt.IsZero() {
		expiryStr = fmt.Sprintf("до %s (%s)", FormatDateMSK(*client.ExpiresAt), daysLeftStr)
	}

	text := fmt.Sprintf(
		"➖ *Сократить:* «*%s*»\n\n"+
			"Текущий срок: %s\n\n"+
			"На сколько дней сократить?",
		client.Name,
		expiryStr,
	)

	markup := &tele.ReplyMarkup{}
	btn7 := markup.Data("-7д", "shorten", "apply", uuid, "7")
	btn30 := markup.Data("-30д", "shorten", "apply", uuid, "30")
	btn90 := markup.Data("-90д", "shorten", "apply", uuid, "90")
	btn365 := markup.Data("-365д", "shorten", "apply", uuid, "365")

	btnCustom := markup.Data("✏️ Другой срок", "shorten", "custom", uuid)
	btnBack := markup.Data("⬅️ К карточке", "client", "view", uuid)

	markup.Inline(
		markup.Row(btn7, btn30),
		markup.Row(btn90, btn365),
		markup.Row(btnCustom),
		markup.Row(btnBack),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ExecuteExtend продлевает срок действия клиента.
func (m *Manager) ExecuteExtend(c tele.Context, uuid string, days int) error {
	updated, err := m.bot.ClientsStorage().Extend(uuid, days)
	if err != nil {
		RespondAlert(c, "Ошибка: "+err.Error())
		return err
	}

	m.fsm.Clear(c.Sender().ID)
	m.bot.LogAdminAction(c.Sender().ID, "extend", uuid, true)

	expStr := "бессрочно"
	if updated.ExpiresAt != nil && !updated.ExpiresAt.IsZero() {
		expStr = FormatDateMSK(*updated.ExpiresAt)
	}

	text := fmt.Sprintf(
		"✅ *%s*: продлён на %d дней (до %s, осталось %s)",
		updated.Name,
		days,
		expStr,
		bot.FormatDaysLeft(updated.ExpiresAt),
	)

	markup := &tele.ReplyMarkup{}
	btnCard := markup.Data("👤 Карточка", "client", "view", uuid)
	btnMain := markup.Data("🏠 В меню", "menu", "main")
	markup.Inline(markup.Row(btnCard, btnMain))

	return c.Send(text, markup, tele.ModeMarkdown)
}

// ExecuteShorten сокращает срок действия клиента.
func (m *Manager) ExecuteShorten(c tele.Context, uuid string, days int, force bool) error {
	client, ok := m.bot.ClientsStorage().Get(uuid)
	if !ok {
		RespondAlert(c, "Клиент не найден")
		return nil
	}

	now := time.Now()
	if !force && client.ExpiresAt != nil && !client.ExpiresAt.IsZero() {
		newExp := client.ExpiresAt.Add(-time.Duration(days) * 24 * time.Hour)
		if newExp.Before(now) {
			// Требуется подтверждение, если срок уходит в прошлое
			pastDays := int(now.Sub(newExp).Hours() / 24)
			text := fmt.Sprintf(
				"⚠️ После сокращения на %d дней срок действия ключа *%s* истечёт в прошлом (примерно %dд назад).\n\n"+
					"Клиент станет просроченным. Продолжить?",
				days,
				client.Name,
				pastDays,
			)

			markup := &tele.ReplyMarkup{}
			btnConfirm := markup.Data("✅ Да, сократить", "shorten", "force", uuid, strconv.Itoa(days))
			btnCancel := markup.Data("✖️ Отмена", "client", "view", uuid)
			markup.Inline(markup.Row(btnConfirm, btnCancel))

			return c.Send(text, markup, tele.ModeMarkdown)
		}
	}

	updated, err := m.bot.ClientsStorage().Extend(uuid, -days)
	if err != nil {
		RespondAlert(c, "Ошибка: "+err.Error())
		return err
	}

	m.fsm.Clear(c.Sender().ID)
	m.bot.LogAdminAction(c.Sender().ID, "shorten", uuid, true)

	expStr := "бессрочно"
	if updated.ExpiresAt != nil && !updated.ExpiresAt.IsZero() {
		expStr = FormatDateMSK(*updated.ExpiresAt)
	}

	text := fmt.Sprintf(
		"✅ *%s*: сокращён на %d дней (до %s, осталось %s)",
		updated.Name,
		days,
		expStr,
		bot.FormatDaysLeft(updated.ExpiresAt),
	)

	markup := &tele.ReplyMarkup{}
	btnCard := markup.Data("👤 Карточка", "client", "view", uuid)
	btnMain := markup.Data("🏠 В меню", "menu", "main")
	markup.Inline(markup.Row(btnCard, btnMain))

	return c.Send(text, markup, tele.ModeMarkdown)
}

// HandleFSMExtendCustomText обрабатывает ввод произвольного количества дней для продления.
func (m *Manager) HandleFSMExtendCustomText(c tele.Context, s *State, text string) error {
	days, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || days <= 0 {
		return c.Send("Пожалуйста, введите положительное целое число дней:")
	}
	if days > bot.MaxDays {
		days = bot.MaxDays
	}
	return m.ExecuteExtend(c, s.ClientID, days)
}

// HandleFSMShortenCustomText обрабатывает ввод произвольного количества дней для сокращения.
func (m *Manager) HandleFSMShortenCustomText(c tele.Context, s *State, text string) error {
	days, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || days <= 0 {
		return c.Send("Пожалуйста, введите положительное целое число дней:")
	}
	return m.ExecuteShorten(c, s.ClientID, days, false)
}
