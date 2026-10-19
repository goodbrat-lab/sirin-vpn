package menu

import (
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"
)

// ShowStats отображает сводную статистику по клиентам.
func (m *Manager) ShowStats(c tele.Context, edit bool) error {
	all := m.bot.ClientsStorage().List()

	now := time.Now()
	sevenDaysAgo := now.Add(-7 * 24 * time.Hour)
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)

	var activeCount, expiringCount, revokedCount, expiredCount int
	var newLast7Days, newLast30Days int

	for _, cl := range all {
		if cl.Revoked {
			revokedCount++
		} else if cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() && cl.ExpiresAt.Before(now) {
			expiredCount++
		} else {
			activeCount++
			if cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() && cl.ExpiresAt.Sub(now) < 7*24*time.Hour {
				expiringCount++
			}
		}

		if cl.CreatedAt.After(sevenDaysAgo) {
			newLast7Days++
		}
		if cl.CreatedAt.After(thirtyDaysAgo) {
			newLast30Days++
		}
	}

	text := fmt.Sprintf(
		"📊 *Статистика Sirin*\n"+
			"─────────────────\n"+
			"Всего клиентов: *%d*\n"+
			"🟢 Активных: *%d*\n"+
			"⏳ Истекают < 7 дней: *%d*\n"+
			"🔴 Отключено: *%d*\n"+
			"⚫ Просрочено: *%d*\n"+
			"─────────────────\n"+
			"Новых за 7 дней: *%d*\n"+
			"Новых за 30 дней: *%d*",
		len(all),
		activeCount,
		expiringCount,
		revokedCount,
		expiredCount,
		newLast7Days,
		newLast30Days,
	)

	markup := &tele.ReplyMarkup{}
	btnExpiring := markup.Data("⏳ Истекают <7д", "clients", "expiring")
	btnRefresh := markup.Data("🔄 Обновить", "menu", "stats")
	btnMain := markup.Data("🏠 В меню", "menu", "main")

	markup.Inline(
		markup.Row(btnExpiring, btnRefresh),
		markup.Row(btnMain),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}
