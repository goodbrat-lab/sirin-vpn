package menu

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
	"sirin/server/internal/bot"
	"sirin/server/internal/relays"
)

// getMSKLocation возвращает часовой пояс Москвы (Europe/Moscow или UTC+3).
func getMSKLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err == nil {
		return loc
	}
	return time.FixedZone("MSK", 3*3600)
}

// BuildDailyReport строит текст подробного ежедневного отчёта о работе системы.
func (m *Manager) BuildDailyReport() (string, *tele.ReplyMarkup) {
	mskLoc := getMSKLocation()
	now := time.Now().In(mskLoc)

	var sb strings.Builder
	sb.WriteString("📊 *Ежедневный отчёт Sirin VPN*\n")
	sb.WriteString(fmt.Sprintf("🕒 *%s, %02d:%02d MSK*\n",
		formatRussianDate(now), now.Hour(), now.Minute()))
	sb.WriteString("────────────────────\n\n")

	// 1. Статус инфраструктуры и VPS-серверов
	sb.WriteString("🖥 *Статус серверов инфраструктуры:*\n")
	reports := make(map[string]bot.MonitorReportItem)
	if m.bot.API() != nil {
		if rep, err := m.bot.API().GetMonitorReports(); err == nil && rep != nil {
			reports = rep
		}
	}

	// 1.1 Нидерланды (NL Primary)
	sb.WriteString("• 🇳🇱 *Нидерланды (NL Primary)*: 🟢 В сети (API / Шлюз)\n")

	// 1.2 Москва (Relay 1)
	moscowRep, hasMoscow := reports["moscow"]
	if !hasMoscow {
		moscowRep, hasMoscow = reports["nl_to_moscow"]
	}
	if hasMoscow {
		statusEmoji := "🟢"
		if moscowRep.Status != "ok" || moscowRep.LossPct > 5 {
			statusEmoji = "🟡"
		}
		sb.WriteString(fmt.Sprintf("• 🇷🇺 *Москва (Relay 1)*: %s RTT: *%.1f мс* (потери *%.0f%%*)\n",
			statusEmoji, moscowRep.RTTMs, moscowRep.LossPct))
	} else {
		sb.WriteString("• 🇷🇺 *Москва (Relay 1)*: 🟢 В сети\n")
	}

	// 1.3 Германия (Relay 2)
	germanyRep, hasGermany := reports["germany"]
	if !hasGermany {
		germanyRep, hasGermany = reports["nl_to_germany"]
	}
	if hasGermany {
		statusEmoji := "🟢"
		if germanyRep.Status != "ok" || germanyRep.LossPct > 5 {
			statusEmoji = "🟡"
		}
		sb.WriteString(fmt.Sprintf("• 🇩🇪 *Германия (Relay 2)*: %s RTT: *%.1f мс* (потери *%.0f%%*)\n\n",
			statusEmoji, germanyRep.RTTMs, germanyRep.LossPct))
	} else {
		sb.WriteString("• 🇩🇪 *Германия (Relay 2)*: 🟢 В сети\n\n")
	}

	// 2. Клиенты и подписки
	sb.WriteString("👥 *Клиенты и подписки:*\n")
	allClients := m.bot.ClientsStorage().List()
	activeCount := 0
	expiredCount := 0
	revokedCount := 0

	type expiringClient struct {
		Name     string
		DaysLeft int
		ExpDate  string
	}
	var expiringList []expiringClient

	for _, c := range allClients {
		if c.Revoked {
			revokedCount++
			continue
		}
		if c.IsActive() {
			activeCount++
			if c.ExpiresAt != nil && !c.ExpiresAt.IsZero() {
				days, ok := c.DaysLeft()
				if ok && days >= 0 && days <= 3 {
					expiringList = append(expiringList, expiringClient{
						Name:     c.Name,
						DaysLeft: days,
						ExpDate:  c.ExpiresAt.In(mskLoc).Format("02.01"),
					})
				}
			}
		} else {
			expiredCount++
		}
	}

	sb.WriteString(fmt.Sprintf("• Всего клиентов: *%d*\n", len(allClients)))
	sb.WriteString(fmt.Sprintf("• Активных подписок: *%d*\n", activeCount))
	if expiredCount > 0 || revokedCount > 0 {
		sb.WriteString(fmt.Sprintf("• Просрочено / отозвано: *%d*\n", expiredCount+revokedCount))
	}

	if len(expiringList) > 0 {
		sort.Slice(expiringList, func(i, j int) bool {
			return expiringList[i].DaysLeft < expiringList[j].DaysLeft
		})
		sb.WriteString("⏳ *Истекают в ближайшие 3 дня:*\n")
		for _, exp := range expiringList {
			sb.WriteString(fmt.Sprintf("  ▫️ *%s* — осталось *%d дн.* (до %s)\n", exp.Name, exp.DaysLeft, exp.ExpDate))
		}
	} else {
		sb.WriteString("• Истекающих в ближайшие 3 дня: *нет*\n")
	}
	sb.WriteString("\n")

	// 3. Серверы-релеи
	if rs := m.bot.RelaysStorage(); rs != nil {
		allRelays := rs.GetAll()
		if len(allRelays) > 0 {
			sb.WriteString("📡 *Нагрузка на релеи:*\n")
			for _, r := range allRelays {
				statusStr := "активен"
				if !r.Enabled {
					statusStr = "выключен"
				}
				cnt := relays.CountActiveOnRelay(allClients, r.ID)
				sb.WriteString(fmt.Sprintf("• %s: *%d/%d* клиентов (%s)\n", r.Name, cnt, r.MaxClients, statusStr))
			}
			sb.WriteString("\n")
		}
	}

	// 4. Поддержка и тикеты
	sb.WriteString("🆘 *Техподдержка и обращения:*\n")
	openTicketsCount := 0
	if ts := m.bot.TicketsStorage(); ts != nil {
		openTickets := ts.GetOpen()
		openTicketsCount = len(openTickets)
		sb.WriteString(fmt.Sprintf("• Открытых обращений: *%d*\n", openTicketsCount))

		if openTicketsCount > 0 {
			for _, t := range openTickets {
				userName := t.Username
				if userName == "" {
					userName = fmt.Sprintf("ID %d", t.TelegramID)
				}
				subj := t.Subject
				if len(t.Messages) > 0 {
					lastMsg := t.Messages[len(t.Messages)-1].Text
					subj = truncateMsg(lastMsg, 35)
				}
				sb.WriteString(fmt.Sprintf("  ▫️ [#%d] *%s* (%s): _%s_\n",
					t.ID, userName, t.DeviceLabel, subj))
			}
		}
	} else {
		sb.WriteString("• Модуль тикетов не подключен\n")
	}
	sb.WriteString("\n")

	// 5. Заявки на триал
	if st := m.bot.Storage(); st != nil {
		pendingTrials := st.ListPendingTrialRequests()
		sb.WriteString(fmt.Sprintf("📋 *Заявки на триал:* *%d* ожидают одобрения\n", len(pendingTrials)))
	}

	sb.WriteString("────────────────────\n")
	sb.WriteString("_Отчёт сформирован автоматически для администратора._")

	markup := &tele.ReplyMarkup{}
	btnClients := markup.Data("👥 Клиенты", "menu", "clients")
	btnTickets := markup.Data(fmt.Sprintf("🆘 Тикеты (%d)", openTicketsCount), "ticket", "adm_open")
	btnRefresh := markup.Data("🔄 Обновить отчёт", "menu", "daily_report")
	btnMain := markup.Data("🏠 Главное меню", "menu", "main")

	markup.Inline(
		markup.Row(btnClients, btnTickets),
		markup.Row(btnRefresh, btnMain),
	)

	return sb.String(), markup
}

// formatRussianDate возвращает красиво отформатированную дату на русском.
func formatRussianDate(t time.Time) string {
	months := [...]string{
		"", "января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	weekdays := [...]string{
		"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота",
	}
	return fmt.Sprintf("%s, %d %s %d г.", weekdays[t.Weekday()], t.Day(), months[t.Month()], t.Year())
}

// ShowDailyReport отправляет или редактирует сообщение с отчётом.
func (m *Manager) ShowDailyReport(c tele.Context, edit bool) error {
	text, markup := m.BuildDailyReport()
	if edit {
		return c.Edit(text, markup, tele.ModeMarkdown)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// SendDailyReportToAdmins рассылает ежедневный отчёт всем администраторам.
func (m *Manager) SendDailyReportToAdmins() {
	adminIDs := m.bot.Config().AdminIDs()
	if len(adminIDs) == 0 {
		return
	}

	text, markup := m.BuildDailyReport()
	tb := m.bot.TeleBot()

	for _, adminID := range adminIDs {
		_, err := tb.Send(&tele.User{ID: adminID}, text, markup, tele.ModeMarkdown)
		if err != nil {
			log.Printf("[SirinBot] Failed to send daily report to admin %d: %v", adminID, err)
		} else {
			log.Printf("[SirinBot] Daily report sent to admin %d successfully", adminID)
		}
	}
}

// StartDailyReportScheduler запускает таймер на 07:00 MSK каждое утро.
func (m *Manager) StartDailyReportScheduler() {
	go func() {
		mskLoc := getMSKLocation()
		for {
			now := time.Now().In(mskLoc)
			// Целевое время — 07:00:00 MSK сегодня
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, mskLoc)
			if !nextRun.After(now) {
				// Если 07:00 сегодня уже прошло, переносим на 07:00 завтра
				nextRun = nextRun.Add(24 * time.Hour)
			}

			waitDuration := nextRun.Sub(now)
			log.Printf("[SirinBot] Daily report scheduled for %s (in %v)",
				nextRun.Format("2006-01-02 15:04:05 MST"), waitDuration.Round(time.Second))

			time.Sleep(waitDuration)

			log.Printf("[SirinBot] 07:00 MSK trigger! Sending daily report...")
			m.SendDailyReportToAdmins()

			// Небольшая пауза, чтобы не сработать дважды за одну секунду
			time.Sleep(10 * time.Second)
		}
	}()
}
