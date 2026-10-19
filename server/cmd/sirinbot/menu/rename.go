package menu

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"sirin/server/internal/bot"
	tele "gopkg.in/telebot.v3"
)

// ShowRenamePrompt отображает приглашение ко вводу нового имени.
func (m *Manager) ShowRenamePrompt(c tele.Context, uuid string, edit bool) error {
	client, ok := m.bot.ClientsStorage().Get(uuid)
	if !ok {
		RespondAlert(c, "Клиент не найден")
		return nil
	}

	m.fsm.Set(c.Sender().ID, "rename_new", uuid, 0, map[string]string{
		"old_name": client.Name,
	})

	text := fmt.Sprintf(
		"✏️ *Переименование — шаг 1/1*\n\n"+
			"Старое имя: «*%s*»\n\n"+
			"Введите новое имя (можно с пробелами, до 64 символов):",
		client.Name,
	)

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "client", "view", uuid)
	markup.Inline(markup.Row(btnCancel))

	if edit {
		return c.Edit(text, markup, tele.ModeMarkdown)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// HandleFSMRenameText применяет новое имя к клиенту и возвращает обновлённую карточку.
func (m *Manager) HandleFSMRenameText(c tele.Context, s *State, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return c.Send("Имя не может быть пустым. Введите новое имя:")
	}

	if utf8.RuneCountInString(text) > bot.MaxNameLen {
		runes := []rune(text)
		text = strings.TrimSpace(string(runes[:bot.MaxNameLen]))
	}
	tokens := strings.Fields(text)
	newName := strings.Join(tokens, " ")

	err := m.bot.ClientsStorage().Rename(s.ClientID, newName)
	if err != nil {
		return c.Send("Ошибка при переименовании: " + err.Error())
	}

	m.fsm.Clear(c.Sender().ID)
	m.bot.LogAdminAction(c.Sender().ID, "rename", s.ClientID, true)

	updated, ok := m.bot.ClientsStorage().Get(s.ClientID)
	if !ok {
		return c.Send(fmt.Sprintf("✅ Переименован в «*%s*»", newName), tele.ModeMarkdown)
	}

	// Выводим обновлённую карточку с сообщением об успехе
	_ = c.Send(fmt.Sprintf("✅ Клиент успешно переименован в «*%s*»", newName), tele.ModeMarkdown)
	return m.ShowClientCard(c, updated, false)
}
