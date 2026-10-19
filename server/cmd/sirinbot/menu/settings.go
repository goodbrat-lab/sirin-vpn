package menu

import (
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

// ShowSettings отображает экран настроек бота.
func (m *Manager) ShowSettings(c tele.Context, edit bool) error {
	cfg := m.bot.Config()

	adminCount := len(cfg.Admin.TelegramIDs)
	if adminCount == 0 && cfg.AdminID != 0 {
		adminCount = 1
	}

	var maskedIDs []string
	if len(cfg.Admin.TelegramIDs) > 0 {
		for _, id := range cfg.Admin.TelegramIDs {
			maskedIDs = append(maskedIDs, MaskTelegramID(id))
		}
	} else if cfg.AdminID != 0 {
		maskedIDs = append(maskedIDs, MaskTelegramID(cfg.AdminID))
	}

	text := fmt.Sprintf(
		"⚙️ *Настройки бота*\n"+
			"─────────────────\n"+
			"Администраторов: *%d* (%s)\n"+
			"Хранилище: `%s`\n"+
			"Уведомления об истечении: *[⬜ Выкл]* _(в разработке)_\n"+
			"Автоочистка просроченных: *[⬜ Выкл]*\n"+
			"─────────────────\n"+
			"_Для добавления администратора отредактируйте /etc/sirin/sirinbot.toml_",
		adminCount,
		strings.Join(maskedIDs, ", "),
		cfg.ClientsPath,
	)

	markup := &tele.ReplyMarkup{}
	btnNotif := markup.Data("🔔 Уведомления", "settings", "notif_todo")
	btnCleanup := markup.Data("🧹 Очистить просроченные", "settings", "cleanup_prompt")
	btnMain := markup.Data("🏠 В меню", "menu", "main")

	markup.Inline(
		markup.Row(btnNotif),
		markup.Row(btnCleanup),
		markup.Row(btnMain),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowCleanupPrompt отображает диалог подтверждения очистки просроченных ключей.
func (m *Manager) ShowCleanupPrompt(c tele.Context, edit bool) error {
	all := m.bot.ClientsStorage().List()
	now := time.Now()
	cutoff := now.Add(-30 * 24 * time.Hour)

	var candidates int
	for _, cl := range all {
		if cl.Revoked || (cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() && cl.ExpiresAt.Before(cutoff)) {
			candidates++
		}
	}

	if candidates == 0 {
		RespondAlert(c, "Нет кандидатов для очистки (отключённых или просроченных >30д)")
		return nil
	}

	text := fmt.Sprintf(
		"⚠️ *Очистка просроченных ключей*\n\n"+
			"Найдено *%d* клиентов (отключённые или просроченные более 30 дней назад).\n\n"+
			"Они будут *безвозвратно удалены* из базы. Продолжить?",
		candidates,
	)

	markup := &tele.ReplyMarkup{}
	btnConfirm := markup.Data("⚠️ Да, удалить просроченные", "settings", "cleanup_confirm")
	btnCancel := markup.Data("✖️ Отмена", "menu", "settings")
	markup.Inline(markup.Row(btnConfirm, btnCancel))

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ExecuteCleanup удаляет отключённых и просроченных >30 дней клиентов.
func (m *Manager) ExecuteCleanup(c tele.Context) error {
	all := m.bot.ClientsStorage().List()
	now := time.Now()
	cutoff := now.Add(-30 * 24 * time.Hour)

	var deletedCount int
	for _, cl := range all {
		if cl.Revoked || (cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() && cl.ExpiresAt.Before(cutoff)) {
			_, err := m.bot.ClientsStorage().Delete(cl.UUID)
			if err == nil {
				deletedCount++
			}
		}
	}

	m.bot.LogAdminAction(c.Sender().ID, "cleanup_expired", fmt.Sprintf("count=%d", deletedCount), true)

	text := fmt.Sprintf("🧹 Очистка завершена: удалено *%d* клиентов.", deletedCount)
	markup := &tele.ReplyMarkup{}
	btnSettings := markup.Data("⚙️ К настройкам", "menu", "settings")
	btnMain := markup.Data("🏠 В меню", "menu", "main")
	markup.Inline(markup.Row(btnSettings, btnMain))

	return EditOrSend(c, text, markup)
}
