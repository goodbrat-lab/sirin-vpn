package menu

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"sirin/server/internal/bot"
	tele "gopkg.in/telebot.v3"
)

const PageSize = 5

// ShowClientsList отображает постраничный список клиентов.
func (m *Manager) ShowClientsList(c tele.Context, page int, filter string, edit bool) error {
	all := m.bot.ClientsStorage().List()

	// Применение фильтра при наличии
	var filtered []*bot.Client
	filterLower := strings.ToLower(strings.TrimSpace(filter))
	for _, client := range all {
		if filterLower == "" {
			filtered = append(filtered, client)
			continue
		}
		if strings.Contains(strings.ToLower(client.Name), filterLower) ||
			strings.Contains(strings.ToLower(client.UUID), filterLower) {
			filtered = append(filtered, client)
		}
	}

	// Сортировка: активные сначала (по возрастанию срока), затем истекающие, затем отключенные и просроченные
	now := time.Now()
	sort.Slice(filtered, func(i, j int) bool {
		ci, cj := filtered[i], filtered[j]
		if ci.Revoked != cj.Revoked {
			return !ci.Revoked // не отозванные первыми
		}
		expI := ci.ExpiresAt != nil && !ci.ExpiresAt.IsZero() && ci.ExpiresAt.Before(now)
		expJ := cj.ExpiresAt != nil && !cj.ExpiresAt.IsZero() && cj.ExpiresAt.Before(now)
		if expI != expJ {
			return !expI // не просроченные первыми
		}
		if ci.ExpiresAt != nil && cj.ExpiresAt != nil {
			return ci.ExpiresAt.Before(*cj.ExpiresAt)
		}
		return strings.ToLower(ci.Name) < strings.ToLower(cj.Name)
	})

	// Статистика
	var activeCount, expiringCount, revokedCount, expiredCount int
	for _, client := range all {
		if client.Revoked {
			revokedCount++
		} else if client.ExpiresAt != nil && !client.ExpiresAt.IsZero() && client.ExpiresAt.Before(now) {
			expiredCount++
		} else {
			activeCount++
			if client.ExpiresAt != nil && !client.ExpiresAt.IsZero() && client.ExpiresAt.Sub(now) < 7*24*time.Hour {
				expiringCount++
			}
		}
	}

	totalItems := len(filtered)
	totalPages := (totalItems + PageSize - 1) / PageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	var text strings.Builder
	text.WriteString(fmt.Sprintf("👥 *Клиенты* (%d)\n", totalItems))
	text.WriteString(fmt.Sprintf("🟢 активных: %d  ⏳ истекают <7д: %d  🔴 отключено: %d", activeCount, expiringCount, revokedCount))
	if expiredCount > 0 {
		text.WriteString(fmt.Sprintf("  ⚫ просрочено: %d", expiredCount))
	}
	text.WriteString("\n")

	if filter != "" {
		text.WriteString(fmt.Sprintf("\n🔍 Фильтр: «%s» (найдено %d из %d)\n", filter, totalItems, len(all)))
	}

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	if totalItems == 0 {
		text.WriteString("\n_Клиенты не найдены._")
	} else {
		start := (page - 1) * PageSize
		end := start + PageSize
		if end > totalItems {
			end = totalItems
		}

		for _, cl := range filtered[start:end] {
			emoji, _ := ClientStatus(cl)
			daysLeft := bot.FormatDaysLeft(cl.ExpiresAt)
			btnText := fmt.Sprintf("%s %s · %s", emoji, cl.Name, daysLeft)
			btn := markup.Data(btnText, "client", "view", cl.UUID)
			rows = append(rows, markup.Row(btn))
		}
	}

	// Навигационная строка
	var prevBtn, countBtn, nextBtn tele.Btn
	if page > 1 {
		prevBtn = markup.Data("◀", "clients", "page", strconv.Itoa(page-1))
	} else {
		prevBtn = markup.Data("◀", "noop", "first")
	}
	countBtn = markup.Data(fmt.Sprintf("%d/%d", page, totalPages), "noop", "page")
	if page < totalPages {
		nextBtn = markup.Data("▶", "clients", "page", strconv.Itoa(page+1))
	} else {
		nextBtn = markup.Data("▶", "noop", "last")
	}
	rows = append(rows, markup.Row(prevBtn, countBtn, nextBtn))

	// Нижняя панель: Фильтр и В меню
	var filterBtn tele.Btn
	if filter != "" {
		filterBtn = markup.Data("✖️ Сбросить фильтр", "clients", "reset_filter")
	} else {
		filterBtn = markup.Data("🔍 Фильтр", "clients", "filter")
	}
	backBtn := markup.Data("⬅️ В меню", "menu", "main")
	rows = append(rows, markup.Row(filterBtn, backBtn))

	markup.Inline(rows...)

	if edit {
		return EditOrSend(c, text.String(), markup)
	}
	return c.Send(text.String(), markup, tele.ModeMarkdown)
}

// ShowClientCard отображает карточку отдельного клиента.
func (m *Manager) ShowClientCard(c tele.Context, client *bot.Client, edit bool) error {
	emoji, statusLabel := ClientStatus(client)
	masked := bot.MaskUUID(client.UUID)
	createdStr := FormatDateMSK(client.CreatedAt)

	expiryStr := "бессрочно"
	if client.ExpiresAt != nil && !client.ExpiresAt.IsZero() {
		daysTotal := int(client.ExpiresAt.Sub(client.CreatedAt).Hours() / 24)
		if daysTotal > 0 {
			expiryStr = fmt.Sprintf("%d дней (до %s)", daysTotal, FormatDateMSK(*client.ExpiresAt))
		} else {
			expiryStr = fmt.Sprintf("до %s", FormatDateMSK(*client.ExpiresAt))
		}
	}

	daysLeftStr := bot.FormatDaysLeft(client.ExpiresAt)

	relayStr := "Прямой NL (Нидерланды)"
	if client.RelayID == 1 {
		relayStr = "Relay 1 (Москва, RU)"
	}

	text := fmt.Sprintf(
		"👤 *%s*\n"+
			"🆔 `%s`\n"+
			"📅 Создан: %s (МСК)\n"+
			"⏱ Срок: %s\n"+
			"📊 Осталось: %s\n"+
			"📡 Узел: %s\n"+
			"%s Статус: %s",
		client.Name,
		masked,
		createdStr,
		expiryStr,
		daysLeftStr,
		relayStr,
		emoji,
		statusLabel,
	)

	markup := &tele.ReplyMarkup{}
	btnShowUUID := markup.Data("👁 Показать UUID", "client", "show_uuid", client.UUID)
	btnRename := markup.Data("✏️ Переименовать", "client", "rename", client.UUID)

	btnExtend := markup.Data("➕ Продлить", "client", "extend", client.UUID)
	btnShorten := markup.Data("➖ Сократить", "client", "shorten", client.UUID)

	var btnToggle tele.Btn
	if client.Revoked {
		btnToggle = markup.Data("🟢 Включить", "client", "enable", client.UUID)
	} else {
		btnToggle = markup.Data("🔴 Отключить", "client", "disable", client.UUID)
	}
	btnDelete := markup.Data("🗑 Удалить", "client", "delete", client.UUID)

	var btnRelay tele.Btn
	if client.RelayID == 1 {
		btnRelay = markup.Data("🌐 Переключить на NL", "client", "set_relay", client.UUID, "0")
	} else {
		btnRelay = markup.Data("📡 Переключить на Relay 1", "client", "set_relay", client.UUID, "1")
	}

	btnBackList := markup.Data("⬅️ К клиентам", "menu", "clients")
	btnBackMain := markup.Data("🏠 В меню", "menu", "main")

	markup.Inline(
		markup.Row(btnShowUUID, btnRename),
		markup.Row(btnExtend, btnShorten),
		markup.Row(btnToggle, btnDelete),
		markup.Row(btnRelay),
		markup.Row(btnBackList, btnBackMain),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// HandleShowUUID отправляет полное сообщение с UUID и таймером скрытия на 30 секунд.
func (m *Manager) HandleShowUUID(c tele.Context, uuid string) error {
	client, ok := m.bot.ClientsStorage().Get(uuid)
	if !ok {
		RespondAlert(c, "Клиент не найден")
		return nil
	}

	text := fmt.Sprintf(
		"⚠️ *UUID — секрет. Не делайте скриншотов!*\n\n"+
			"Клиент: *%s*\n"+
			"`%s`\n\n"+
			"_Сообщение автоматически скроется через 30 секунд._",
		client.Name,
		client.UUID,
	)

	markup := &tele.ReplyMarkup{}
	btnHide := markup.Data("🙈 Скрыть сейчас", "uuid", "hide", client.UUID)
	markup.Inline(markup.Row(btnHide))

	msg, err := m.bot.TeleBot().Send(c.Recipient(), text, markup, tele.ModeMarkdown)
	if err != nil {
		return err
	}

	// Автоскрытие через 30 секунд
	time.AfterFunc(30*time.Second, func() {
		hideMarkup := &tele.ReplyMarkup{}
		btnShowAgain := hideMarkup.Data("👁 Показать снова", "client", "show_uuid", client.UUID)
		hideMarkup.Inline(hideMarkup.Row(btnShowAgain))

		_, _ = m.bot.TeleBot().Edit(
			msg,
			"🔒 *UUID скрыт в целях безопасности.*",
			hideMarkup,
			tele.ModeMarkdown,
		)
	})

	return nil
}

// HandleHideUUID немедленно скрывает показанный UUID.
func (m *Manager) HandleHideUUID(c tele.Context, uuid string) error {
	markup := &tele.ReplyMarkup{}
	btnShowAgain := markup.Data("👁 Показать снова", "client", "show_uuid", uuid)
	markup.Inline(markup.Row(btnShowAgain))

	return c.Edit("🔒 *UUID скрыт.*", markup, tele.ModeMarkdown)
}
