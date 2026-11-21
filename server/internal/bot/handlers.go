package bot

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

func (b *Bot) handleStart(c tele.Context) error {
	userID := c.Sender().ID
	text := "🕊 *Сирин* — Stay unseen\n\n" +
		"Я помогу подключиться к VPN.\n\n" +
		"Команды:\n" +
		"/mykey — показать мой выданный ключ\n" +
		"/devices — привязанные устройства\n" +
		"/help — справка\n"

	if b.isAdmin(userID) {
		text += "\n👑 *Команды администратора:*\n" +
			"/newkey `<имя> [дни]` — создать ключ\n" +
			"/list `[фильтр]` — список клиентов\n" +
			"/info `<имя|uuid>` — карточка клиента\n" +
			"/find `<запрос>` — быстрый поиск\n" +
			"/rename `<старое> -> <новое>` — переименовать\n" +
			"/extend `<имя|uuid> <дни>` — продлить\n" +
			"/shorten `<имя|uuid> <дни>` — сократить срок\n" +
			"/disable `<имя|uuid>` — отключить (soft-delete)\n" +
			"/enable `<имя|uuid>` — включить\n" +
			"/unbind `<имя|uuid>` — отвязать все устройства\n" +
			"/maxdevices `<имя|uuid> <N>` — лимит устройств\n" +
			"/delete `<имя|uuid>` — удалить ключ\n" +
			"/status — статистика сервера\n"
	}

	text += "\nДля подключения введи свой UUID в приложении «Сирин»."
	return c.Send(text, tele.ModeMarkdown)
}

func (b *Bot) handleHelp(c tele.Context) error {
	userID := c.Sender().ID
	text := "📖 *Справка*\n\n" +
		"`/mykey` — показать текущий ключ\n" +
		"`/devices` — привязанные устройства и отвязка\n"

	if b.isAdmin(userID) {
		text += "\n👑 *Управление клиентами:*\n" +
			"• `/newkey Марина Иванова 365` — создать ключ на 365 дней\n" +
			"• `/list` — постраничный список клиентов и статистика\n" +
			"• `/list Тест` — фильтр списка по имени\n" +
			"• `/info Марина` — полная информация и кнопки управления\n" +
			"• `/find Иванова` — быстрый переход к клиенту\n" +
			"• `/rename Марина Иванова -> Марина И.` — переименовать\n" +
			"• `/extend Марина 30` — продлить срок на 30 дней\n" +
			"• `/shorten Марина 10` — сократить срок на 10 дней\n" +
			"• `/disable Марина` — временно отключить клиента\n" +
			"• `/enable Марина` — активировать отключенного клиента\n" +
			"• `/unbind Марина` — сбросить привязки устройств\n" +
			"• `/maxdevices Марина 3` — установить лимит устройств (семья)\n" +
			"• `/delete Марина` — удалить клиента (с подтверждением)\n" +
			"• `/delete Марина --force` — удалить немедленно\n" +
			"• `/status` — статистика сервера и API\n"
	}
	return c.Send(text, tele.ModeMarkdown)
}

func (b *Bot) handleMyKey(c tele.Context) error {
	userID := c.Sender().ID
	uk, ok := b.storage.Get(userID)
	if !ok {
		return c.Send("У тебя пока нет ключа. Обратитесь к администратору для получения доступа.")
	}

	cl, clOk := b.clientsStorage.Get(uk.UUID)
	devicesCount := 0
	maxDevices := 1
	expiryText := "бессрочно"
	if clOk && cl != nil {
		if cl.MaxDevices > 0 {
			maxDevices = cl.MaxDevices
		}
		if len(cl.DeviceIDs) > 0 {
			devicesCount = len(cl.DeviceIDs)
		} else if cl.DeviceID != "" {
			devicesCount = 1
		}
		if cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() {
			days, _ := cl.DaysLeft()
			if days < 0 {
				expiryText = fmt.Sprintf("истёк %s ⚠️", cl.ExpiresAt.Format("02.01.2006"))
			} else {
				expiryText = fmt.Sprintf("до %s (осталось %d дн.)", cl.ExpiresAt.Format("02.01.2006"), days)
			}
		}
	}

	openURL := fmt.Sprintf("https://crp.guseynovia.ru/key?uuid=%s", uk.UUID)
	btn := &tele.ReplyMarkup{}
	btnOpen := btn.URL("🕊 Открыть в Сирин", openURL)
	btnQR := btn.Data("📷 QR-код", "my_qr:"+uk.UUID)
	btnDevices := btn.Data("📱 Мои устройства", "my_dev:"+uk.UUID)
	btnStars := btn.Data("⭐ Продлить через Stars", "my_stars:"+uk.UUID)
	btn.Inline(
		btn.Row(btnOpen),
		btn.Row(btnQR, btnDevices),
		btn.Row(btnStars),
	)

	openwrtCmd := fmt.Sprintf("wget -O- https://crp.guseynovia.ru/install.sh | sh -s -- %s", uk.UUID)
	text := fmt.Sprintf("🔑 *Твой ключ доступа*\n\n"+
		"Имя: `%s`\n"+
		"Ключ: `%s`\n"+
		"Срок: *%s*\n"+
		"Устройств: `%d / %d`\n\n"+
		"📱 *На телефоне:*\n"+
		"Нажмите на кнопку *«🕊 Открыть в Сирин»* ниже, чтобы активировать ключ в 1 клик, либо откройте *«📷 QR-код»*.\n\n"+
		"💻 *На роутере OpenWrt (вставьте в терминал):*\n"+
		"`%s`\n\n"+
		"[Открыть в Сирин](%s)", uk.Name, maskUUID(uk.UUID), expiryText, devicesCount, maxDevices, openwrtCmd, openURL)

	return c.Send(text, btn, tele.ModeMarkdown)
}

func (b *Bot) handleTariffs(c tele.Context) error {
	userID := c.Sender().ID
	keyUUID := "new"
	if uk, ok := b.storage.Get(userID); ok && uk != nil && uk.UUID != "" {
		keyUUID = uk.UUID
	}

	text := "💳 *Тарифы Sirin VPN*\n\n" +
		"Выберите период для мгновенной оплаты через *Telegram Stars*:\n\n" +
		"• *1 месяц (30 дн.)* — 75 ⭐ / 150 ₽\n" +
		"• *3 месяца (90 дн.)* — 200 ⭐ / 400 ₽ _(скидка 11%)_\n" +
		"• *6 месяцев (180 дн.)* — 375 ⭐ / 750 ₽ _(скидка 17%)_\n" +
		"• *1 год (365 дн.)* — 700 ⭐ / 1 350 ₽ _(скидка 25%)_\n\n" +
		"⚡ _Мгновенная активация, безлимитный трафик, поддержка всех устройств._"

	markup := &tele.ReplyMarkup{}
	btn1m := markup.Data("1 мес — 75 ⭐", "stars_buy:1m:"+keyUUID)
	btn3m := markup.Data("3 мес — 200 ⭐", "stars_buy:3m:"+keyUUID)
	btn6m := markup.Data("6 мес — 375 ⭐", "stars_buy:6m:"+keyUUID)
	btn1y := markup.Data("1 год — 700 ⭐", "stars_buy:1y:"+keyUUID)

	markup.Inline(
		markup.Row(btn1m, btn3m),
		markup.Row(btn6m, btn1y),
	)

	return c.Send(text, markup, tele.ModeMarkdown)
}

func (b *Bot) handleDevices(c tele.Context) error {
	userID := c.Sender().ID
	uk, ok := b.storage.Get(userID)
	if !ok {
		return c.Send("У тебя пока нет ключа. Обратитесь к администратору для получения доступа.")
	}

	cl, clOk := b.clientsStorage.Get(uk.UUID)
	devicesCount := 0
	maxDevices := 1
	var devList []string
	if clOk && cl != nil {
		if cl.MaxDevices > 0 {
			maxDevices = cl.MaxDevices
		}
		if len(cl.DeviceIDs) > 0 {
			devicesCount = len(cl.DeviceIDs)
			devList = cl.DeviceIDs
		} else if cl.DeviceID != "" {
			devicesCount = 1
			devList = []string{cl.DeviceID}
		}
	}

	var sb strings.Builder
	sb.WriteString("📱 *Привязанные устройства*\n\n")
	sb.WriteString(fmt.Sprintf("Ключ: `%s`\n", maskUUID(uk.UUID)))
	sb.WriteString(fmt.Sprintf("Лимит: *%d / %d*\n\n", devicesCount, maxDevices))

	if len(devList) == 0 {
		sb.WriteString("Сейчас к ключу не привязано ни одного устройства.\n")
	} else {
		sb.WriteString("Подключенные устройства:\n")
		for i, id := range devList {
			label := id
			if len(label) > 16 {
				label = label[:16] + "…"
			}
			sb.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, label))
		}
		sb.WriteString("\nЕсли вы сменили телефон или роутер и достигли лимита, нажмите кнопку ниже, чтобы сбросить привязки:")
	}

	btn := &tele.ReplyMarkup{}
	if len(devList) > 0 {
		btnUnbind := btn.Data("🔄 Отвязать все устройства", "my_unbind:"+uk.UUID)
		btn.Inline(btn.Row(btnUnbind))
		return c.Send(sb.String(), btn, tele.ModeMarkdown)
	}

	return c.Send(sb.String(), tele.ModeMarkdown)
}

func (b *Bot) handleStatus(c tele.Context) error {
	health, err := b.api.Health()
	if err != nil {
		return c.Send(fmt.Sprintf("❌ API недоступен: %v", err))
	}

	clients := b.clientsStorage.List()
	activeCount := 0
	for _, cl := range clients {
		if cl.IsActive() {
			activeCount++
		}
	}

	text := fmt.Sprintf("📊 *Статус сервера*\n\n"+
		"API: `%v`\n"+
		"Всего клиентов в базе: `%d`\n"+
		"Активных ключей: `%d`", health["status"], len(clients), activeCount)
	return c.Send(text, tele.ModeMarkdown)
}

func (b *Bot) handleNewKey(c tele.Context) error {
	userID := c.Sender().ID
	rawInput := c.Data()
	if rawInput == "" {
		rawInput = c.Text()
	}

	name, days, clamped, err := parseNewKeyArgsDetailed(rawInput)
	if err != nil {
		return c.Send("Укажи имя: `/newkey <имя> [дни]`\nПример: `/newkey Марина Иванова 365`", tele.ModeMarkdown)
	}

	resp, err := b.api.CreateClient(name, days)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Ошибка API: %v", err))
	}

	_ = b.storage.Set(UserKey{
		TelegramID: userID,
		UUID:       resp.Key,
		Name:       name,
		CreatedAt:  time.Now().Unix(),
	})

	b.logAdminAction(userID, "newkey", resp.Key[:8], true)

	openURL := fmt.Sprintf("https://crp.guseynovia.ru/key?uuid=%s", resp.Key)
	btn := &tele.ReplyMarkup{}
	btnOpen := btn.URL("🕊 Открыть в Сирин", openURL)
	btn.Inline(btn.Row(btnOpen))

	durationText := fmt.Sprintf("%d дней", days)
	if clamped {
		durationText = fmt.Sprintf("%d дней (срок ограничен 3650)", days)
	}

	// Полный UUID не показываем в чате для защиты от скриншотов
	text := fmt.Sprintf("✅ *Ключ создан*\n\n"+
		"Имя: `%s`\n"+
		"Срок: %s\n"+
		"🔑 Ключ: `%s`\n\n"+
		"Нажми на кнопку ниже, чтобы открыть приложение «Сирин» и применить ключ:\n"+
		"[Открыть в Сирин](%s)", name, durationText, maskUUID(resp.Key), openURL)

	return c.Send(text, btn, tele.ModeMarkdown)
}

// -----------------------------------------------------------------------------
// /list [фильтр]
// -----------------------------------------------------------------------------

func (b *Bot) handleList(c tele.Context) error {
	raw := strings.TrimSpace(c.Data())
	text, markup := b.buildListPage(1, raw)
	return c.Send(text, markup, tele.ModeMarkdown)
}

func (b *Bot) buildListPage(page int, filter string) (string, *tele.ReplyMarkup) {
	allClients := b.clientsStorage.List()

	// Фильтрация
	var filtered []*Client
	filterLower := strings.ToLower(strings.TrimSpace(filter))
	for _, cl := range allClients {
		if filterLower == "" || strings.Contains(strings.ToLower(cl.Name), filterLower) {
			filtered = append(filtered, cl)
		}
	}

	// Сортировка: сначала активные (не отозваны и не просрочены), затем по дате истечения
	sort.Slice(filtered, func(i, j int) bool {
		actI := filtered[i].IsActive()
		actJ := filtered[j].IsActive()
		if actI != actJ {
			return actI // активные первыми
		}
		// Если оба активны или оба неактивны: сортируем по ExpiresAt
		if filtered[i].ExpiresAt == nil {
			return false
		}
		if filtered[j].ExpiresAt == nil {
			return true
		}
		return filtered[i].ExpiresAt.Before(*filtered[j].ExpiresAt)
	})

	// Статистика по всей базе
	totalAll := len(allClients)
	activeAll := 0
	expiringSoon := 0
	expiredCount := 0
	disabledCount := 0
	now := time.Now()

	for _, cl := range allClients {
		if cl.Revoked {
			disabledCount++
		} else if cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() {
			if cl.ExpiresAt.Before(now) {
				expiredCount++
			} else {
				activeAll++
				if cl.ExpiresAt.Sub(now) < 7*24*time.Hour {
					expiringSoon++
				}
			}
		} else {
			activeAll++ // бессрочный
		}
	}

	const pageSize = 20
	totalFiltered := len(filtered)
	totalPages := (totalFiltered + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if end > totalFiltered {
		end = totalFiltered
	}

	var sb strings.Builder
	if filterLower != "" {
		sb.WriteString(fmt.Sprintf("📋 *Клиенты (фильтр: «%s»)* [стр. %d/%d]\n\n", filter, page, totalPages))
	} else {
		sb.WriteString(fmt.Sprintf("📋 *Список клиентов* [стр. %d/%d]\n\n", page, totalPages))
	}

	if totalFiltered == 0 {
		sb.WriteString("_Ничего не найдено._\n")
	} else {
		for _, cl := range filtered[start:end] {
			statusIcon := "✅"
			if cl.Revoked {
				statusIcon = "⛔"
			} else if cl.ExpiresAt != nil && cl.ExpiresAt.Before(now) {
				statusIcon = "⏰"
			}

			daysLeftStr := formatDaysLeft(cl.ExpiresAt)
			expStr := "бессрочно"
			if cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() {
				expStr = cl.ExpiresAt.Format("2006-01-02")
			}

			sb.WriteString(fmt.Sprintf("• *%s* · `%s` · %s · до %s · %s\n",
				escapeMarkdown(cl.Name), maskUUID(cl.UUID), daysLeftStr, expStr, statusIcon))
		}
	}

	sb.WriteString(fmt.Sprintf("\n📊 Всего: %d · Активных: %d · Истекают <7д: %d · Просрочены: %d · Отключены: %d",
		totalAll, activeAll, expiringSoon, expiredCount, disabledCount))

	markup := &tele.ReplyMarkup{}
	var navRows []tele.Row

	if totalPages > 1 {
		var buttons []tele.Btn
		if page > 1 {
			buttons = append(buttons, markup.Data("◀ Назад", fmt.Sprintf("page:%d:%s", page-1, filter)))
		}
		buttons = append(buttons, markup.Data(fmt.Sprintf("%d/%d", page, totalPages), "noop"))
		if page < totalPages {
			buttons = append(buttons, markup.Data("Вперёд ▶", fmt.Sprintf("page:%d:%s", page+1, filter)))
		}
		navRows = append(navRows, markup.Row(buttons...))
	}

	markup.Inline(navRows...)
	return sb.String(), markup
}

// -----------------------------------------------------------------------------
// /info <name|prefix>
// -----------------------------------------------------------------------------

func (b *Bot) handleInfo(c tele.Context) error {
	arg := strings.TrimSpace(c.Data())
	if arg == "" {
		return c.Send("Использование: `/info <имя|префикс_uuid>`\nПример: `/info Марина`", tele.ModeMarkdown)
	}

	all := b.clientsStorage.List()
	client, err, ambiguous := resolveClient(all, arg)
	if err != nil {
		if err == ErrAmbiguousClient {
			return b.renderAmbiguousSelection(c, ambiguous, "info")
		}
		return c.Send(fmt.Sprintf("Клиент «%s» не найден. Используйте `/list` для просмотра списка.", arg), tele.ModeMarkdown)
	}

	text, markup := b.buildInfoCard(client)
	return c.Send(text, markup, tele.ModeMarkdown)
}

func (b *Bot) buildInfoCard(cl *Client) (string, *tele.ReplyMarkup) {
	status := "✅ активен"
	if cl.Revoked {
		status = "⛔ отключён"
	} else if cl.ExpiresAt != nil && cl.ExpiresAt.Before(time.Now()) {
		status = "⏰ просрочен"
	}

	createdStr := cl.CreatedAt.Format("2006-01-02")
	expStr := "бессрочно"
	if cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() {
		expStr = cl.ExpiresAt.Format("2006-01-02")
	}
	daysLeftStr := formatDaysLeft(cl.ExpiresAt)

	devicesCount := 0
	maxDevices := 1
	if cl.MaxDevices > 0 {
		maxDevices = cl.MaxDevices
	}
	if len(cl.DeviceIDs) > 0 {
		devicesCount = len(cl.DeviceIDs)
	} else if cl.DeviceID != "" {
		devicesCount = 1
	}

	text := fmt.Sprintf("👤 *%s*\n\n"+
		"UUID: `%s`\n"+
		"Создан: %s\n"+
		"Срок: до %s\n"+
		"Осталось: %s\n"+
		"Устройств: `%d / %d`\n"+
		"Статус: %s",
		escapeMarkdown(cl.Name), maskUUID(cl.UUID), createdStr, expStr, daysLeftStr, devicesCount, maxDevices, status)

	markup := &tele.ReplyMarkup{}
	btnShow := markup.Data("👁 Показать UUID", "show:"+cl.UUID)
	btnExt := markup.Data("➕ Продлить (+30д)", "ext30:"+cl.UUID)
	btnShort := markup.Data("➖ Сократить (-30д)", "short30:"+cl.UUID)

	var btnToggle tele.Btn
	if cl.Revoked {
		btnToggle = markup.Data("✅ Включить", "ena:"+cl.UUID)
	} else {
		btnToggle = markup.Data("⛔ Отключить", "dis:"+cl.UUID)
	}
	btnDel := markup.Data("🗑 Удалить", "del_req:"+cl.UUID)
	btnUnbind := markup.Data("📱 Сбросить устройства", "unbind:"+cl.UUID)

	markup.Inline(
		markup.Row(btnShow),
		markup.Row(btnExt, btnShort),
		markup.Row(btnToggle, btnDel),
		markup.Row(btnUnbind),
	)

	return text, markup
}

func (b *Bot) renderAmbiguousSelection(c tele.Context, clients []*Client, action string) error {
	var sb strings.Builder
	sb.WriteString("⚠️ Найдено несколько совпадений. Выберите нужного клиента:\n\n")

	markup := &tele.ReplyMarkup{}
	var rows []tele.Row

	for _, cl := range clients {
		sb.WriteString(fmt.Sprintf("• %s (%s)\n", cl.Name, maskUUID(cl.UUID)))
		btn := markup.Data(fmt.Sprintf("%s (%s)", cl.Name, cl.UUID[:8]), fmt.Sprintf("%s:%s", action, cl.UUID))
		rows = append(rows, markup.Row(btn))
	}

	markup.Inline(rows...)
	return c.Send(sb.String(), markup)
}

// -----------------------------------------------------------------------------
// /rename <old> -> <new>
// -----------------------------------------------------------------------------

func (b *Bot) handleRename(c tele.Context) error {
	userID := c.Sender().ID
	raw := c.Data()
	if raw == "" {
		raw = c.Text()
	}

	oldName, newName, err := parseRenameArgs(raw)
	if err != nil {
		return c.Send("Использование: `/rename <старое имя> -> <новое имя>`\nПример: `/rename Марина Иванова -> Марина И.`", tele.ModeMarkdown)
	}

	all := b.clientsStorage.List()
	client, err, ambiguous := resolveClient(all, oldName)
	if err != nil {
		if err == ErrAmbiguousClient {
			return b.renderAmbiguousSelection(c, ambiguous, "info")
		}
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", oldName))
	}

	// Предупреждение, если имя совпадает с другим клиентом
	duplicateWarning := ""
	for _, cl := range all {
		if cl.UUID != client.UUID && strings.EqualFold(cl.Name, newName) {
			duplicateWarning = fmt.Sprintf("\n⚠️ _Предупреждение: имя «%s» уже используется другим клиентом._", newName)
			break
		}
	}

	if err := b.clientsStorage.Rename(client.UUID, newName); err != nil {
		return c.Send(fmt.Sprintf("❌ Ошибка при переименовании: %v", err))
	}

	b.logAdminAction(userID, "rename", client.UUID[:8], true)
	return c.Send(fmt.Sprintf("✅ Переименован: *%s* → *%s*%s", escapeMarkdown(client.Name), escapeMarkdown(newName), duplicateWarning), tele.ModeMarkdown)
}

// -----------------------------------------------------------------------------
// /extend & /shorten
// -----------------------------------------------------------------------------

func (b *Bot) handleExtend(c tele.Context) error {
	return b.processExtend(c, false)
}

func (b *Bot) handleShorten(c tele.Context) error {
	return b.processExtend(c, true)
}

func (b *Bot) processExtend(c tele.Context, isShorten bool) error {
	userID := c.Sender().ID
	fields := strings.Fields(c.Data())
	if len(fields) < 2 {
		cmdName := "/extend"
		if isShorten {
			cmdName = "/shorten"
		}
		return c.Send(fmt.Sprintf("Использование: `%s <имя|uuid> <дни>`\nПример: `%s Марина 30`", cmdName, cmdName), tele.ModeMarkdown)
	}

	lastToken := fields[len(fields)-1]
	days, err := strconv.Atoi(lastToken)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Некорректное число дней: «%s»", lastToken))
	}

	if isShorten {
		if days > 0 {
			days = -days
		}
	}

	// Clamp -3650..3650
	if days > 3650 {
		days = 3650
	} else if days < -3650 {
		days = -3650
	}

	target := strings.Join(fields[:len(fields)-1], " ")
	all := b.clientsStorage.List()
	client, err, ambiguous := resolveClient(all, target)
	if err != nil {
		if err == ErrAmbiguousClient {
			return b.renderAmbiguousSelection(c, ambiguous, "info")
		}
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", target))
	}

	updated, err := b.clientsStorage.Extend(client.UUID, days)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Ошибка продления: %v", err))
	}

	b.logAdminAction(userID, "extend", client.UUID[:8], true)

	expStr := "бессрочно"
	if updated.ExpiresAt != nil {
		expStr = updated.ExpiresAt.Format("2006-01-02")
	}

	return c.Send(fmt.Sprintf("✅ *%s*: срок изменён на %+d дн. (до %s, осталось: %s)",
		escapeMarkdown(updated.Name), days, expStr, formatDaysLeft(updated.ExpiresAt)), tele.ModeMarkdown)
}

// -----------------------------------------------------------------------------
// /disable & /enable
// -----------------------------------------------------------------------------

func (b *Bot) handleDisable(c tele.Context) error {
	userID := c.Sender().ID
	arg := strings.TrimSpace(c.Data())
	if arg == "" {
		return c.Send("Использование: `/disable <имя|uuid>`", tele.ModeMarkdown)
	}

	all := b.clientsStorage.List()
	client, err, ambiguous := resolveClient(all, arg)
	if err != nil {
		if err == ErrAmbiguousClient {
			return b.renderAmbiguousSelection(c, ambiguous, "info")
		}
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", arg))
	}

	if _, err := b.clientsStorage.SetRevoked(client.UUID, true); err != nil {
		return c.Send(fmt.Sprintf("❌ Ошибка: %v", err))
	}

	b.logAdminAction(userID, "disable", client.UUID[:8], true)
	return c.Send(fmt.Sprintf("⛔ *%s* отключён. Сессии будут прерваны при следующей синхронизации (до 30 сек).", escapeMarkdown(client.Name)), tele.ModeMarkdown)
}

func (b *Bot) handleEnable(c tele.Context) error {
	userID := c.Sender().ID
	arg := strings.TrimSpace(c.Data())
	if arg == "" {
		return c.Send("Использование: `/enable <имя|uuid>`", tele.ModeMarkdown)
	}

	all := b.clientsStorage.List()
	client, err, ambiguous := resolveClient(all, arg)
	if err != nil {
		if err == ErrAmbiguousClient {
			return b.renderAmbiguousSelection(c, ambiguous, "info")
		}
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", arg))
	}

	updated, err := b.clientsStorage.SetRevoked(client.UUID, false)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Ошибка: %v", err))
	}

	b.logAdminAction(userID, "enable", client.UUID[:8], true)

	// Проверка: если истёк срок
	if updated.ExpiresAt != nil && updated.ExpiresAt.Before(time.Now()) {
		markup := &tele.ReplyMarkup{}
		btnExt := markup.Data("➕ Продлить на 30 дней", "ext30:"+updated.UUID)
		markup.Inline(markup.Row(btnExt))
		return c.Send(fmt.Sprintf("✅ *%s* включён.\n⏰ _Внимание: срок действия истёк (%s). Продлить?_",
			escapeMarkdown(updated.Name), formatDaysLeft(updated.ExpiresAt)), markup, tele.ModeMarkdown)
	}

	return c.Send(fmt.Sprintf("✅ *%s* включён.", escapeMarkdown(updated.Name)), tele.ModeMarkdown)
}

// -----------------------------------------------------------------------------
// /delete <name|prefix> [--force]
// -----------------------------------------------------------------------------

func (b *Bot) handleDelete(c tele.Context) error {
	userID := c.Sender().ID
	raw := c.Data()
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return c.Send("Использование: `/delete <имя|uuid> [--force]`", tele.ModeMarkdown)
	}

	isForce := false
	var targetParts []string
	for _, f := range fields {
		if f == "--force" {
			isForce = true
		} else {
			targetParts = append(targetParts, f)
		}
	}

	target := strings.Join(targetParts, " ")
	all := b.clientsStorage.List()
	client, err, ambiguous := resolveClient(all, target)
	if err != nil {
		if err == ErrAmbiguousClient {
			return b.renderAmbiguousSelection(c, ambiguous, "info")
		}
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", target))
	}

	if isForce {
		if _, err := b.clientsStorage.Delete(client.UUID); err != nil {
			return c.Send(fmt.Sprintf("❌ Ошибка удаления: %v", err))
		}
		b.logAdminAction(userID, "delete_force", client.UUID[:8], true)
		return c.Send(fmt.Sprintf("🗑 Клиент *%s* удалён. Активные сессии будут прерваны при следующей синхронизации.", escapeMarkdown(client.Name)), tele.ModeMarkdown)
	}

	// Подтверждение через inline-кнопку с таймаутом 60 секунд
	b.pendingDelMu.Lock()
	b.pendingDeletes[client.UUID] = pendingDelete{
		UUID:      client.UUID,
		ExpiresAt: time.Now().Add(60 * time.Second),
	}
	b.pendingDelMu.Unlock()

	activeWarning := ""
	if client.IsActive() {
		activeWarning = "\n⚠️ _Внимание: клиент активен! Удаление разорвёт активную сессию._"
	}

	markup := &tele.ReplyMarkup{}
	btnConfirm := markup.Data("🗑 Да, удалить", "del_cfm:"+client.UUID)
	btnCancel := markup.Data("Отмена", "del_ccl:"+client.UUID)
	markup.Inline(markup.Row(btnConfirm, btnCancel))

	text := fmt.Sprintf("🗑 Вы уверены, что хотите удалить клиента *%s* (`%s`)?%s\n\n_Подтверждение действует 60 секунд._",
		escapeMarkdown(client.Name), maskUUID(client.UUID), activeWarning)

	return c.Send(text, markup, tele.ModeMarkdown)
}

// -----------------------------------------------------------------------------
// /find <query>
// -----------------------------------------------------------------------------

func (b *Bot) handleFind(c tele.Context) error {
	arg := strings.TrimSpace(c.Data())
	if arg == "" {
		return c.Send("Использование: `/find <запрос>`\nПример: `/find Марина`", tele.ModeMarkdown)
	}

	all := b.clientsStorage.List()
	client, err, _ := resolveClient(all, arg)
	if err == nil {
		text, markup := b.buildInfoCard(client)
		return c.Send(text, markup, tele.ModeMarkdown)
	}

	if err == ErrAmbiguousClient {
		text, markup := b.buildListPage(1, arg)
		return c.Send(text, markup, tele.ModeMarkdown)
	}

	return c.Send(fmt.Sprintf("По запросу «%s» ничего не найдено. Используйте `/list` для просмотра полного списка.", arg))
}

// -----------------------------------------------------------------------------
// /unbind <name|prefix>
// -----------------------------------------------------------------------------

func (b *Bot) handleUnbind(c tele.Context) error {
	userID := c.Sender().ID
	arg := strings.TrimSpace(c.Data())
	if arg == "" {
		return c.Send("Использование: `/unbind <имя|uuid>`", tele.ModeMarkdown)
	}

	all := b.clientsStorage.List()
	client, err, ambiguous := resolveClient(all, arg)
	if err != nil {
		if err == ErrAmbiguousClient {
			return b.renderAmbiguousSelection(c, ambiguous, "info")
		}
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", arg))
	}

	updated, err := b.clientsStorage.UnbindDevice(client.UUID, "")
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Ошибка отвязки: %v", err))
	}

	b.logAdminAction(userID, "unbind", client.UUID[:8], true)
	return c.Send(fmt.Sprintf("✅ Все устройства клиента *%s* отвязаны.", escapeMarkdown(updated.Name)), tele.ModeMarkdown)
}

// -----------------------------------------------------------------------------
// /maxdevices <name|prefix> <N>
// -----------------------------------------------------------------------------

func (b *Bot) handleMaxDevices(c tele.Context) error {
	userID := c.Sender().ID
	fields := strings.Fields(c.Data())
	if len(fields) < 2 {
		return c.Send("Использование: `/maxdevices <имя|uuid> <лимит (1..20)>`", tele.ModeMarkdown)
	}

	limitStr := fields[len(fields)-1]
	target := strings.Join(fields[:len(fields)-1], " ")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 20 {
		return c.Send("Лимит должен быть числом от 1 до 20.")
	}

	all := b.clientsStorage.List()
	client, err, ambiguous := resolveClient(all, target)
	if err != nil {
		if err == ErrAmbiguousClient {
			return b.renderAmbiguousSelection(c, ambiguous, "info")
		}
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", target))
	}

	updated, err := b.clientsStorage.SetMaxDevices(client.UUID, limit)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Ошибка установки лимита: %v", err))
	}

	b.logAdminAction(userID, "max_devices", fmt.Sprintf("%s:%d", client.UUID[:8], limit), true)
	return c.Send(fmt.Sprintf("✅ Для *%s* установлен лимит: *%d* устройств.", escapeMarkdown(updated.Name), updated.MaxDevices), tele.ModeMarkdown)
}

// -----------------------------------------------------------------------------
// Обработка inline-кнопок
// -----------------------------------------------------------------------------

func (b *Bot) handleCallback(c tele.Context) error {
	userID := c.Sender().ID
	data := strings.TrimSpace(c.Callback().Data)

	if data == "noop" {
		return c.Respond()
	}

	// 1. Постраничная навигация list: page:<num>:<filter>
	if strings.HasPrefix(data, "page:") {
		if !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступно только администратору", ShowAlert: true})
		}
		parts := strings.SplitN(data, ":", 3)
		if len(parts) >= 2 {
			page, _ := strconv.Atoi(parts[1])
			filter := ""
			if len(parts) == 3 {
				filter = parts[2]
			}
			text, markup := b.buildListPage(page, filter)
			return c.Edit(text, markup, tele.ModeMarkdown)
		}
		return c.Respond()
	}

	// 2. Показать карточку клиента info:<uuid>
	if strings.HasPrefix(data, "info:") {
		if !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступно только администратору", ShowAlert: true})
		}
		uuidKey := strings.TrimPrefix(data, "info:")
		cl, ok := b.clientsStorage.Get(uuidKey)
		if !ok {
			return c.Respond(&tele.CallbackResponse{Text: "Клиент не найден", ShowAlert: true})
		}
		text, markup := b.buildInfoCard(cl)
		_ = c.Respond()
		return c.Send(text, markup, tele.ModeMarkdown)
	}

	// 3. Показать полный UUID с предупреждением show:<uuid>
	if strings.HasPrefix(data, "show:") {
		if !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступно только администратору", ShowAlert: true})
		}
		uuidKey := strings.TrimPrefix(data, "show:")
		cl, ok := b.clientsStorage.Get(uuidKey)
		if !ok {
			return c.Respond(&tele.CallbackResponse{Text: "Клиент не найден", ShowAlert: true})
		}
		b.logAdminAction(userID, "show_uuid", uuidKey[:8], true)
		return c.Respond(&tele.CallbackResponse{
			Text:      fmt.Sprintf("⚠️ НЕ ДЕЛАЙТЕ СКРИНШОТОВ!\n\nUUID клиента %s:\n%s", cl.Name, cl.UUID),
			ShowAlert: true,
		})
	}

	// 4. Быстрое продление ext30:<uuid>
	if strings.HasPrefix(data, "ext30:") {
		if !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступно только администратору", ShowAlert: true})
		}
		uuidKey := strings.TrimPrefix(data, "ext30:")
		updated, err := b.clientsStorage.Extend(uuidKey, 30)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка: " + err.Error(), ShowAlert: true})
		}
		b.logAdminAction(userID, "extend_30", uuidKey[:8], true)
		_ = c.Respond(&tele.CallbackResponse{Text: "✅ Продлён на 30 дней"})
		text, markup := b.buildInfoCard(updated)
		return c.Edit(text, markup, tele.ModeMarkdown)
	}

	// 5. Сокращение на 30 дней short30:<uuid>
	if strings.HasPrefix(data, "short30:") {
		if !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступно только администратору", ShowAlert: true})
		}
		uuidKey := strings.TrimPrefix(data, "short30:")
		updated, err := b.clientsStorage.Extend(uuidKey, -30)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка: " + err.Error(), ShowAlert: true})
		}
		b.logAdminAction(userID, "shorten_30", uuidKey[:8], true)
		_ = c.Respond(&tele.CallbackResponse{Text: "✅ Срок сокращён на 30 дней"})
		text, markup := b.buildInfoCard(updated)
		return c.Edit(text, markup, tele.ModeMarkdown)
	}

	// 6. Отключение dis:<uuid>
	if strings.HasPrefix(data, "dis:") {
		if !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступно только администратору", ShowAlert: true})
		}
		uuidKey := strings.TrimPrefix(data, "dis:")
		updated, err := b.clientsStorage.SetRevoked(uuidKey, true)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка: " + err.Error(), ShowAlert: true})
		}
		b.logAdminAction(userID, "disable", uuidKey[:8], true)
		_ = c.Respond(&tele.CallbackResponse{Text: "⛔ Клиент отключён"})
		text, markup := b.buildInfoCard(updated)
		return c.Edit(text, markup, tele.ModeMarkdown)
	}

	// 7. Включение ena:<uuid>
	if strings.HasPrefix(data, "ena:") {
		if !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступно только администратору", ShowAlert: true})
		}
		uuidKey := strings.TrimPrefix(data, "ena:")
		updated, err := b.clientsStorage.SetRevoked(uuidKey, false)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка: " + err.Error(), ShowAlert: true})
		}
		b.logAdminAction(userID, "enable", uuidKey[:8], true)
		_ = c.Respond(&tele.CallbackResponse{Text: "✅ Клиент включён"})
		text, markup := b.buildInfoCard(updated)
		return c.Edit(text, markup, tele.ModeMarkdown)
	}

	// 8. Запрос на удаление del_req:<uuid>
	if strings.HasPrefix(data, "del_req:") {
		if !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступно только администратору", ShowAlert: true})
		}
		uuidKey := strings.TrimPrefix(data, "del_req:")
		cl, ok := b.clientsStorage.Get(uuidKey)
		if !ok {
			return c.Respond(&tele.CallbackResponse{Text: "Клиент не найден", ShowAlert: true})
		}

		b.pendingDelMu.Lock()
		b.pendingDeletes[cl.UUID] = pendingDelete{
			UUID:      cl.UUID,
			ExpiresAt: time.Now().Add(60 * time.Second),
		}
		b.pendingDelMu.Unlock()

		markup := &tele.ReplyMarkup{}
		btnConfirm := markup.Data("🗑 Да, удалить", "del_cfm:"+cl.UUID)
		btnCancel := markup.Data("Отмена", "del_ccl:"+cl.UUID)
		markup.Inline(markup.Row(btnConfirm, btnCancel))

		_ = c.Respond()
		return c.Send(fmt.Sprintf("🗑 Подтвердите удаление *%s* (`%s`).\n_Подтверждение действует 60 секунд._",
			escapeMarkdown(cl.Name), maskUUID(cl.UUID)), markup, tele.ModeMarkdown)
	}

	// 9. Подтверждение удаления del_cfm:<uuid>
	if strings.HasPrefix(data, "del_cfm:") {
		if !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступно только администратору", ShowAlert: true})
		}
		uuidKey := strings.TrimPrefix(data, "del_cfm:")

		b.pendingDelMu.Lock()
		p, exists := b.pendingDeletes[uuidKey]
		delete(b.pendingDeletes, uuidKey)
		b.pendingDelMu.Unlock()

		if !exists || time.Now().After(p.ExpiresAt) {
			_ = c.Respond(&tele.CallbackResponse{Text: "⏰ Время ожидания подтверждения истекло (60 сек).", ShowAlert: true})
			return c.Edit("⏰ Время ожидания подтверждения истекло. Повторите попытку.")
		}

		deleted, err := b.clientsStorage.Delete(uuidKey)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка: " + err.Error(), ShowAlert: true})
		}

		b.logAdminAction(userID, "delete_confirmed", uuidKey[:8], true)
		_ = c.Respond(&tele.CallbackResponse{Text: "🗑 Клиент удалён"})
		return c.Edit(fmt.Sprintf("🗑 Клиент *%s* успешно удалён из базы.", escapeMarkdown(deleted.Name)), tele.ModeMarkdown)
	}

	// 10. Отмена удаления del_ccl:<uuid>
	if strings.HasPrefix(data, "del_ccl:") {
		uuidKey := strings.TrimPrefix(data, "del_ccl:")
		b.pendingDelMu.Lock()
		delete(b.pendingDeletes, uuidKey)
		b.pendingDelMu.Unlock()

		_ = c.Respond(&tele.CallbackResponse{Text: "Удаление отменено"})
		return c.Edit("❌ Удаление клиента отменено.")
	}

	// 11. Отвязка устройств пользователем my_unbind:<uuid>
	if strings.HasPrefix(data, "my_unbind:") {
		uuidKey := strings.TrimPrefix(data, "my_unbind:")
		uk, ok := b.storage.Get(userID)
		if (!ok || uk.UUID != uuidKey) && !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступ запрещён", ShowAlert: true})
		}
		if _, err := b.clientsStorage.UnbindDevice(uuidKey, ""); err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка: " + err.Error(), ShowAlert: true})
		}
		return c.Respond(&tele.CallbackResponse{
			Text:      "✅ Все устройства отвязаны! Теперь вы можете подключить новое устройство.",
			ShowAlert: true,
		})
	}

	// 12. Переход к списку устройств пользователя my_dev:<uuid>
	if strings.HasPrefix(data, "my_dev:") {
		_ = c.Respond()
		return b.handleDevices(c)
	}

	// 13. Отвязка устройств администратором unbind:<uuid>
	if strings.HasPrefix(data, "unbind:") {
		if !b.isAdmin(userID) {
			return c.Respond(&tele.CallbackResponse{Text: "⛔ Доступно только администратору", ShowAlert: true})
		}
		uuidKey := strings.TrimPrefix(data, "unbind:")
		updated, err := b.clientsStorage.UnbindDevice(uuidKey, "")
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка: " + err.Error(), ShowAlert: true})
		}
		b.logAdminAction(userID, "unbind", uuidKey[:8], true)
		_ = c.Respond(&tele.CallbackResponse{Text: "✅ Устройства отвязаны"})
		text, markup := b.buildInfoCard(updated)
		return c.Edit(text, markup, tele.ModeMarkdown)
	}

	// 14. Продление через Stars my_stars:<uuid>
	if strings.HasPrefix(data, "my_stars:") {
		uuidKey := strings.TrimPrefix(data, "my_stars:")
		text := "⭐ *Продление подписки Sirin VPN через Telegram Stars*\n\n" +
			"Выберите тариф для моментального продления доступа:\n\n" +
			"• *1 месяц (30 дн.)* — 75 ⭐\n" +
			"• *3 месяца (90 дн.)* — 200 ⭐ _(скидка 11%)_\n" +
			"• *6 месяцев (180 дн.)* — 375 ⭐ _(скидка 17%)_\n" +
			"• *1 год (365 дн.)* — 700 ⭐ _(скидка 25%)_\n\n" +
			"⚡ _После оплаты подписка будет автоматически продлена в ту же секунду._"

		markup := &tele.ReplyMarkup{}
		btn1m := markup.Data("1 мес — 75 ⭐", "stars_buy:1m:"+uuidKey)
		btn3m := markup.Data("3 мес — 200 ⭐", "stars_buy:3m:"+uuidKey)
		btn6m := markup.Data("6 мес — 375 ⭐", "stars_buy:6m:"+uuidKey)
		btn1y := markup.Data("1 год — 700 ⭐", "stars_buy:1y:"+uuidKey)

		markup.Inline(
			markup.Row(btn1m, btn3m),
			markup.Row(btn6m, btn1y),
		)
		_ = c.Respond()
		return c.Send(text, markup, tele.ModeMarkdown)
	}

	// 15. Покупка тарифа через Stars stars_buy:<tariff_id>:<uuid>
	if strings.HasPrefix(data, "stars_buy:") {
		parts := strings.Split(strings.TrimPrefix(data, "stars_buy:"), ":")
		if len(parts) >= 2 {
			tariffID := parts[0]
			uuidKey := parts[1]
			if tariff, ok := FindTariff(tariffID); ok {
				_ = c.Respond()
				return b.SendStarsInvoice(c.Recipient(), tariff, uuidKey)
			}
		}
		return c.Respond(&tele.CallbackResponse{Text: "Тариф не найден", ShowAlert: true})
	}

	// 16. Показ QR-кода ключа my_qr:<uuid>
	if strings.HasPrefix(data, "my_qr:") {
		uuidKey := strings.TrimPrefix(data, "my_qr:")
		openURL := fmt.Sprintf("https://crp.guseynovia.ru/key?uuid=%s", uuidKey)
		pngBytes, err := GenerateKeyQR(openURL)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка генерации QR", ShowAlert: true})
		}

		photo := &tele.Photo{
			File: tele.FromReader(bytes.NewReader(pngBytes)),
		}

		caption := fmt.Sprintf(
			"🕊 *QR-код для подключения к Sirin VPN*\n\n"+
				"📱 *На телефоне:*\n"+
				"• Отсканируйте этот QR-код камерой или встроенным сканером в приложении\n"+
				"• Либо нажмите кнопку *«🕊 Открыть в Сирин»* ниже\n\n"+
				"💻 *На роутере OpenWrt (1 строка в терминале):*\n"+
				"`wget -O- https://crp.guseynovia.ru/install.sh | sh -s -- %s`\n\n"+
				"🔑 *Ваш ключ:* `%s`",
			uuidKey, maskUUID(uuidKey),
		)

		markup := &tele.ReplyMarkup{}
		btnOpen := markup.URL("🕊 Открыть в Сирин", openURL)
		btnDevs := markup.Data("📱 Мои устройства", "my_dev:"+uuidKey)
		markup.Inline(markup.Row(btnOpen), markup.Row(btnDevs))

		_ = c.Respond()
		return c.Send(photo, caption, markup, tele.ModeMarkdown)
	}

	return c.Respond()
}

// -----------------------------------------------------------------------------
// Telegram Stars: OnCheckout & OnPayment
// -----------------------------------------------------------------------------

func (b *Bot) handleCheckout(c tele.Context) error {
	pcq := c.PreCheckoutQuery()
	if pcq == nil {
		return nil
	}
	if strings.HasPrefix(pcq.Payload, "sirin_stars:") {
		return c.Accept()
	}
	return c.Accept("Неизвестный тип заказа")
}

func (b *Bot) handlePayment(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.Payment == nil {
		return nil
	}
	payment := msg.Payment
	if !strings.HasPrefix(payment.Payload, "sirin_stars:") {
		return nil
	}

	parts := strings.Split(payment.Payload, ":")
	if len(parts) < 4 {
		log.Printf("[SirinBot] invalid stars payload: %s", payment.Payload)
		return nil
	}
	clientUUID := parts[1]
	tariffID := parts[2]
	days, _ := strconv.Atoi(parts[3])

	userID := c.Sender().ID
	userName := strings.TrimSpace(c.Sender().FirstName + " " + c.Sender().LastName)
	if userName == "" {
		userName = fmt.Sprintf("User_%d", userID)
	}

	var updated *Client
	var err error

	if clientUUID == "" || clientUUID == "new" {
		// Создание нового клиента
		resp, createErr := b.api.CreateClient(userName, days)
		if createErr != nil {
			log.Printf("[SirinBot] failed to create client after stars payment: %v", createErr)
			return c.Send(fmt.Sprintf("⚠️ Оплата (%d ⭐) получена, но произошла ошибка создания ключа: %v. Администратор оповещён.", payment.Total, createErr))
		}
		clientUUID = resp.Key
		_ = b.storage.Set(UserKey{
			TelegramID: userID,
			UUID:       clientUUID,
			Name:       userName,
			CreatedAt:  time.Now().Unix(),
		})
		updated, _ = b.clientsStorage.Get(clientUUID)
	} else {
		// Продление существующего клиента
		updated, err = b.clientsStorage.Extend(clientUUID, days)
		if err != nil {
			log.Printf("[SirinBot] failed to extend client %s: %v", clientUUID, err)
			return c.Send(fmt.Sprintf("⚠️ Оплата (%d ⭐) получена, но произошла ошибка продления ключа: %v. Администратор оповещён.", payment.Total, err))
		}
		if updated != nil && updated.Revoked {
			_, _ = b.clientsStorage.SetRevoked(clientUUID, false)
			updated, _ = b.clientsStorage.Get(clientUUID)
		}
	}

	// Сброс флагов напоминаний об истечении срока
	if uk, ok := b.storage.Get(userID); ok {
		uk.NotifiedExpiry3dAt = 0
		uk.NotifiedExpiry1dAt = 0
		uk.NotifiedExpiredAt = 0
		_ = b.storage.Set(*uk)
	}

	expiryStr := "бессрочно"
	if updated != nil && updated.ExpiresAt != nil && !updated.ExpiresAt.IsZero() {
		expiryStr = updated.ExpiresAt.Format("02.01.2006")
	}

	openURL := fmt.Sprintf("https://crp.guseynovia.ru/key?uuid=%s", clientUUID)
	markup := &tele.ReplyMarkup{}
	btnOpen := markup.URL("🕊 Открыть в Сирин", openURL)
	btnDevs := markup.Data("📱 Мои устройства", "my_dev:"+clientUUID)
	markup.Inline(markup.Row(btnOpen), markup.Row(btnDevs))

	tariffName := tariffID
	if t, ok := FindTariff(tariffID); ok {
		tariffName = t.Name
	}

	text := fmt.Sprintf(
		"🎉 *Оплата успешно получена!*\n\n"+
			"Тариф: *%s* (%d дней, %d ⭐)\n"+
			"Подписка активна до: *%s*\n"+
			"Ключ: `%s`\n\n"+
			"Спасибо за доверие к Sirin VPN! Нажмите кнопку ниже, чтобы открыть приложение:",
		tariffName, days, payment.Total, expiryStr, maskUUID(clientUUID),
	)

	// Уведомление администраторам
	for _, adminID := range b.cfg.AdminIDs() {
		adminText := fmt.Sprintf(
			"⭐ *Новая оплата через Telegram Stars!*\n\n"+
				"Пользователь: %s (ID: `%d`, @%s)\n"+
				"Тариф: *%s* (%d дней)\n"+
				"Оплачено: *%d ⭐* (Telegram Charge ID: `%s`)\n"+
				"Ключ: `%s` (до %s)",
			userName, userID, c.Sender().Username,
			tariffName, days, payment.Total, payment.TelegramChargeID,
			clientUUID, expiryStr,
		)
		_, _ = b.bot.Send(&tele.User{ID: adminID}, adminText, tele.ModeMarkdown)
	}

	return c.Send(text, markup, tele.ModeMarkdown)
}

func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"`", "\\`",
		"[", "\\[",
	)
	return replacer.Replace(s)
}
