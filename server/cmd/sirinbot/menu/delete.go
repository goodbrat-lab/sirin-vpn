package menu

import (
	"fmt"
	"time"

	"sirin/server/internal/bot"
	tele "gopkg.in/telebot.v3"
)

// ShowDeleteConfirmation выводит диалог подтверждения удаления клиента с таймаутом 60 секунд.
func (m *Manager) ShowDeleteConfirmation(c tele.Context, uuid string, edit bool) error {
	client, ok := m.bot.ClientsStorage().Get(uuid)
	if !ok {
		RespondAlert(c, "Клиент не найден")
		return nil
	}

	text := fmt.Sprintf(
		"🗑 *Удалить клиента?*\n\n"+
			"👤 *%s*\n"+
			"🆔 `%s`\n\n"+
			"⚠️ *Это действие необратимо!* Ключ будет удалён из базы, доступ прекращён.",
		client.Name,
		bot.MaskUUID(client.UUID),
	)

	markup := &tele.ReplyMarkup{}
	btnConfirm := markup.Data("✅ Да, удалить", "delete", "confirm", uuid)
	btnCancel := markup.Data("✖️ Отмена", "client", "view", uuid)
	markup.Inline(markup.Row(btnConfirm, btnCancel))

	var msg *tele.Message
	var err error
	if edit {
		err = c.Edit(text, markup, tele.ModeMarkdown)
		msg = c.Message()
	} else {
		msg, err = m.bot.TeleBot().Send(c.Recipient(), text, markup, tele.ModeMarkdown)
	}
	if err != nil {
		return err
	}

	// Таймаут подтверждения: через 60 секунд убираем кнопки
	if msg != nil {
		time.AfterFunc(60*time.Second, func() {
			timeoutMarkup := &tele.ReplyMarkup{}
			btnMain := timeoutMarkup.Data("🏠 В меню", "menu", "main")
			timeoutMarkup.Inline(timeoutMarkup.Row(btnMain))

			_, _ = m.bot.TeleBot().Edit(
				msg,
				"⏱ *Время подтверждения истекло.*\nДействие отменено.",
				timeoutMarkup,
				tele.ModeMarkdown,
			)
		})
	}

	return nil
}

// ExecuteDelete удаляет клиента из базы данных и выводит результат.
func (m *Manager) ExecuteDelete(c tele.Context, uuid string) error {
	deleted, err := m.bot.ClientsStorage().Delete(uuid)
	if err != nil {
		RespondAlert(c, "Ошибка при удалении: "+err.Error())
		return err
	}

	m.bot.LogAdminAction(c.Sender().ID, "delete", uuid, true)

	text := fmt.Sprintf(
		"✅ Удалён клиент: *%s*\n🆔 `%s`",
		deleted.Name,
		bot.MaskUUID(deleted.UUID),
	)

	markup := &tele.ReplyMarkup{}
	btnClients := markup.Data("👥 К клиентам", "menu", "clients")
	btnMain := markup.Data("🏠 В меню", "menu", "main")
	markup.Inline(markup.Row(btnClients, btnMain))

	return EditOrSend(c, text, markup)
}
