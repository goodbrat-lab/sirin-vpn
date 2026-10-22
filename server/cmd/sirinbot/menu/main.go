package menu

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"sirin/server/internal/bot"
	tele "gopkg.in/telebot.v3"
)

// Manager управляет интерактивным меню, навигацией и FSM.
type Manager struct {
	bot               *bot.Bot
	fsm               *FSM
	dedup             *CallbackDedup
	persistentReplyKb bool
}

// Register создаёт и регистрирует интерактивное меню на боте.
func Register(b *bot.Bot) *Manager {
	m := &Manager{
		bot:               b,
		fsm:               NewFSM(5 * time.Minute),
		dedup:             NewCallbackDedup(),
		persistentReplyKb: false,
	}

	m.registerRoutes()
	return m
}

// registerRoutes регистрирует обработчики меню и обратную совместимость команд.
func (m *Manager) registerRoutes() {
	tb := m.bot.TeleBot()

	// Публичные команды
	tb.Handle("/start", m.handleStart)
	tb.Handle("/help", m.handleHelp)
	tb.Handle("/menu", m.handleMenuCommand)
	tb.Handle("/apps", func(c tele.Context) error {
		return m.ShowClientApps(c, false)
	})
	tb.Handle("/mykey", func(c tele.Context) error {
		return m.ShowClientKey(c, false)
	})

	// Старые админ-команды (обратная совместимость)
	tb.Handle("/list", m.bot.AdminOnly(m.handleListCommand))
	tb.Handle("/info", m.bot.AdminOnly(m.handleInfoCommand))
	tb.Handle("/newkey", m.bot.AdminOnly(m.handleNewKeyCommand))
	tb.Handle("/rename", m.bot.AdminOnly(m.handleRenameCommand))
	tb.Handle("/extend", m.bot.AdminOnly(m.handleExtendCommand))
	tb.Handle("/shorten", m.bot.AdminOnly(m.handleShortenCommand))
	tb.Handle("/disable", m.bot.AdminOnly(m.handleDisableCommand))
	tb.Handle("/enable", m.bot.AdminOnly(m.handleEnableCommand))
	tb.Handle("/delete", m.bot.AdminOnly(m.handleDeleteCommand))
	tb.Handle("/find", m.bot.AdminOnly(m.handleFindCommand))
	tb.Handle("/status", m.bot.AdminOnly(m.handleStatusCommand))

	// Перехват текстовых сообщений для FSM и ReplyKeyboard
	tb.Handle(tele.OnText, m.handleText)

	// Перехват фото и документов для загрузки чеков об оплате
	tb.Handle(tele.OnPhoto, m.HandleReceiptUploaded)
	tb.Handle(tele.OnDocument, m.HandleReceiptUploaded)

	// Центральный диспетчер callback-запросов (inline-кнопки)
	tb.Handle(tele.OnCallback, m.handleCallback)
}

// ShowMainMenu выводит главное меню с актуальной статистикой в заголовке.
func (m *Manager) ShowMainMenu(c tele.Context, edit bool) error {
	m.fsm.Clear(c.Sender().ID)

	all := m.bot.ClientsStorage().List()
	now := time.Now()

	var activeCount, expiredCount int
	for _, cl := range all {
		if !cl.Revoked && (cl.ExpiresAt == nil || cl.ExpiresAt.IsZero() || cl.ExpiresAt.After(now)) {
			activeCount++
		} else if cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() && cl.ExpiresAt.Before(now) {
			expiredCount++
		}
	}

	text := fmt.Sprintf(
		"👋 *Sirin Bot — панель управления*\n\n"+
			"Клиентов: *%d* · Активных: *%d* · Просрочено: *%d*",
		len(all),
		activeCount,
		expiredCount,
	)

	markup := &tele.ReplyMarkup{}
	btnClients := markup.Data("👥 Клиенты", "menu", "clients")
	btnNew := markup.Data("➕ Новый клиент", "menu", "new")

	btnStats := markup.Data("📊 Статистика", "menu", "stats")
	btnSearch := markup.Data("🔍 Поиск", "menu", "search")

	btnSettings := markup.Data("⚙️ Настройки", "menu", "settings")
	btnHelp := markup.Data("ℹ️ Помощь", "menu", "help")

	markup.Inline(
		markup.Row(btnClients, btnNew),
		markup.Row(btnStats, btnSearch),
		markup.Row(btnSettings, btnHelp),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}

	// Опциональная постоянная Reply-клавиатура
	if m.persistentReplyKb {
		replyKb := &tele.ReplyMarkup{ResizeKeyboard: true}
		r1 := replyKb.Text("👥 Клиенты")
		r2 := replyKb.Text("➕ Новый")
		r3 := replyKb.Text("📊 Статистика")
		r4 := replyKb.Text("🔍 Поиск")
		replyKb.Reply(
			replyKb.Row(r1, r2),
			replyKb.Row(r3, r4),
		)
		_ = c.Send("Меню быстрого доступа активировано:", replyKb)
	}

	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowHelp отображает справку по использованию бота.
func (m *Manager) ShowHelp(c tele.Context, edit bool) error {
	text := "ℹ️ *Sirin Bot — справка*\n\n" +
		"Основное управление осуществляется через интерактивные кнопки меню.\n\n" +
		"*Команды быстрого доступа (алиасы):*\n" +
		"• `/menu` — главное меню\n" +
		"• `/list [фильтр]` — список клиентов\n" +
		"• `/newkey <имя> [дни]` — создать ключ\n" +
		"• `/info <имя|uuid>` — карточка клиента\n" +
		"• `/extend <имя|uuid> <дни>` — продлить срок\n" +
		"• `/shorten <имя|uuid> <дни>` — сократить срок\n" +
		"• `/rename <старое> -> <новое>` — переименовать\n" +
		"• `/disable <имя|uuid>` / `/enable <имя|uuid>` — отключить/включить\n" +
		"• `/delete <имя|uuid> [--force]` — удалить ключ\n" +
		"• `/find <запрос>` — быстрый поиск\n" +
		"• `/status` — статистика сервера\n\n" +
		"_Для подключения скопируйте UUID в приложение «Сирин»._"

	markup := &tele.ReplyMarkup{}
	btnMain := markup.Data("🏠 В меню", "menu", "main")
	markup.Inline(markup.Row(btnMain))

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowSearchPrompt выводит приглашение к поиску клиентов.
func (m *Manager) ShowSearchPrompt(c tele.Context, edit bool) error {
	m.fsm.Set(c.Sender().ID, "search_query", "", 0, nil)

	text := "🔍 *Поиск клиентов*\n\n" +
		"Введите имя или префикс UUID (≥6 символов):"

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "menu", "main")
	markup.Inline(markup.Row(btnCancel))

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// handleCallback — центральный роутер inline-кнопок.
func (m *Manager) handleCallback(c tele.Context) error {
	category, action, args := ExtractCallback(c)
	raw := ""
	if c.Callback() != nil {
		raw = c.Callback().Data
	}

	log.Printf("[CALLBACK] from=%d user=%q unique=%q raw=%q => category=%q action=%q args=%v",
		c.Sender().ID, c.Sender().Username, c.Callback().Unique, raw, category, action, args)

	// Дедупликация быстрых двойных кликов
	dedupKey := fmt.Sprintf("%d:%s", c.Sender().ID, raw)
	if !m.dedup.ShouldProcess(dedupKey, 300*time.Millisecond) {
		RespondSafe(c)
		return nil
	}

	// Разграничение доступа: категории личного кабинета доступны всем, остальное — только админам
	clientCategories := map[string]bool{
		"my":    true,
		"trial": true,
		"order": true,
	}
	if !clientCategories[category] && !m.bot.IsAdmin(c.Sender().ID) {
		log.Printf("[CALLBACK DENY] user %d is not admin (cat=%q)", c.Sender().ID, category)
		RespondAlert(c, "⛔ Доступно только администратору.")
		return nil
	}

	RespondSafe(c)

	switch category {
	case "noop":
		if action == "first" {
			RespondAlert(c, "Уже первая страница")
		} else if action == "last" {
			RespondAlert(c, "Уже последняя страница")
		}
		return nil

	case "menu":
		switch action {
		case "main":
			return m.ShowMainMenu(c, true)
		case "clients":
			return m.ShowClientsList(c, 1, "", true)
		case "new":
			return m.StartNewClientWizard(c, true)
		case "stats":
			return m.ShowStats(c, true)
		case "search":
			return m.ShowSearchPrompt(c, true)
		case "settings":
			return m.ShowSettings(c, true)
		case "help":
			return m.ShowHelp(c, true)
		}

	case "clients":
		switch action {
		case "page":
			if len(args) > 0 {
				p, _ := strconv.Atoi(args[0])
				return m.ShowClientsList(c, p, "", true)
			}
		case "filter":
			m.fsm.Set(c.Sender().ID, "clients_filter", "", 0, nil)
			text := "🔍 *Фильтр списка клиентов*\n\nВведите строку для поиска по имени:"
			markup := &tele.ReplyMarkup{}
			btnCancel := markup.Data("✖️ Отмена", "menu", "clients")
			markup.Inline(markup.Row(btnCancel))
			return c.Edit(text, markup, tele.ModeMarkdown)
		case "reset_filter":
			return m.ShowClientsList(c, 1, "", true)
		case "expiring":
			// Фильтр клиентов, истекающих <7д
			all := m.bot.ClientsStorage().List()
			now := time.Now()
			var expiring []*bot.Client
			for _, cl := range all {
				if !cl.Revoked && cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() &&
					cl.ExpiresAt.After(now) && cl.ExpiresAt.Sub(now) < 7*24*time.Hour {
					expiring = append(expiring, cl)
				}
			}
			if len(expiring) == 0 {
				RespondAlert(c, "Нет клиентов, истекающих в ближайшие 7 дней")
				return nil
			}
			return m.ShowClientsList(c, 1, expiring[0].Name, true)
		}

	case "client":
		if len(args) == 0 {
			return nil
		}
		uuid := args[0]
		switch action {
		case "view":
			cl, ok := m.bot.ClientsStorage().Get(uuid)
			if !ok {
				RespondAlert(c, "Клиент не найден")
				return m.ShowClientsList(c, 1, "", true)
			}
			return m.ShowClientCard(c, cl, true)
		case "show_uuid":
			return m.HandleShowUUID(c, uuid)
		case "rename":
			return m.ShowRenamePrompt(c, uuid, true)
		case "extend":
			return m.ShowExtendPrompt(c, uuid, true)
		case "shorten":
			return m.ShowShortenPrompt(c, uuid, true)
		case "disable":
			_, err := m.bot.ClientsStorage().SetRevoked(uuid, true)
			if err != nil {
				RespondAlert(c, "Ошибка: "+err.Error())
				return nil
			}
			m.bot.LogAdminAction(c.Sender().ID, "disable", uuid, true)
			cl, _ := m.bot.ClientsStorage().Get(uuid)
			return m.ShowClientCard(c, cl, true)
		case "enable":
			_, err := m.bot.ClientsStorage().SetRevoked(uuid, false)
			if err != nil {
				RespondAlert(c, "Ошибка: "+err.Error())
				return nil
			}
			m.bot.LogAdminAction(c.Sender().ID, "enable", uuid, true)
			cl, _ := m.bot.ClientsStorage().Get(uuid)
			return m.ShowClientCard(c, cl, true)
		case "delete":
			return m.ShowDeleteConfirmation(c, uuid, true)
		case "link":
			deepLink, webLink := BuildDeepLink(DefaultWebLinkHost, uuid)
			text := fmt.Sprintf(
				"🔗 *Ссылки для подключения:*\n\n"+
					"Прямой deep-link:\n`%s`\n\n"+
					"Веб-ссылка (редирект):\n`%s`",
				deepLink,
				webLink,
			)
			markup := &tele.ReplyMarkup{}
			btnOpen := markup.URL("🕊 Открыть в Сирин", webLink)
			btnCard := markup.Data("👤 К карточке", "client", "view", uuid)
			markup.Inline(
				markup.Row(btnOpen),
				markup.Row(btnCard),
			)
			return c.Send(text, markup, tele.ModeMarkdown)
		}

	case "uuid":
		if action == "hide" && len(args) > 0 {
			return m.HandleHideUUID(c, args[0])
		}

	case "new":
		switch action {
		case "days":
			if len(args) > 0 {
				state, ok := m.fsm.Get(c.Sender().ID)
				name := "Новый клиент"
				if ok && state.Data["name"] != "" {
					name = state.Data["name"]
				}
				if args[0] == "custom" {
					return m.ShowNewClientCustomDays(c, name, true)
				}
				days, _ := strconv.Atoi(args[0])
				return m.ShowNewClientStep3(c, name, days, true)
			}
		case "back":
			if len(args) > 0 {
				step := args[0]
				state, ok := m.fsm.Get(c.Sender().ID)
				name := "Новый клиент"
				if ok && state.Data["name"] != "" {
					name = state.Data["name"]
				}
				if step == "1" {
					return m.StartNewClientWizard(c, true)
				}
				if step == "2" {
					return m.ShowNewClientStep2(c, name, true)
				}
			}
		case "confirm":
			state, ok := m.fsm.Get(c.Sender().ID)
			if !ok || state.Data["name"] == "" {
				RespondAlert(c, "Сессия истекла. Начните заново.")
				return m.ShowMainMenu(c, true)
			}
			name := state.Data["name"]
			days, _ := strconv.Atoi(state.Data["days"])
			if days <= 0 {
				days = 30
			}
			return m.ExecuteCreateClient(c, name, days)
		}

	case "extend":
		if len(args) >= 1 {
			uuid := args[0]
			if action == "apply" && len(args) >= 2 {
				days, _ := strconv.Atoi(args[1])
				return m.ExecuteExtend(c, uuid, days)
			}
			if action == "custom" {
				m.fsm.Set(c.Sender().ID, "extend_days", uuid, 0, nil)
				text := "➕ Введите количество дней для продления:"
				markup := &tele.ReplyMarkup{}
				btnCancel := markup.Data("✖️ Отмена", "client", "view", uuid)
				markup.Inline(markup.Row(btnCancel))
				return c.Edit(text, markup, tele.ModeMarkdown)
			}
		}

	case "shorten":
		if len(args) >= 1 {
			uuid := args[0]
			if action == "apply" && len(args) >= 2 {
				days, _ := strconv.Atoi(args[1])
				return m.ExecuteShorten(c, uuid, days, false)
			}
			if action == "force" && len(args) >= 2 {
				days, _ := strconv.Atoi(args[1])
				return m.ExecuteShorten(c, uuid, days, true)
			}
			if action == "custom" {
				m.fsm.Set(c.Sender().ID, "shorten_days", uuid, 0, nil)
				text := "➖ Введите количество дней для сокращения:"
				markup := &tele.ReplyMarkup{}
				btnCancel := markup.Data("✖️ Отмена", "client", "view", uuid)
				markup.Inline(markup.Row(btnCancel))
				return c.Edit(text, markup, tele.ModeMarkdown)
			}
		}

	case "delete":
		if action == "confirm" && len(args) > 0 {
			return m.ExecuteDelete(c, args[0])
		}

	case "settings":
		switch action {
		case "notif_todo":
			RespondAlert(c, "Уведомления об истечении подписок находятся в разработке.")
			return nil
		case "cleanup_prompt":
			return m.ShowCleanupPrompt(c, true)
		case "cleanup_confirm":
			return m.ExecuteCleanup(c)
		}

	case "my":
		switch action {
		case "home":
			return m.ShowClientHome(c, true)
		case "key":
			return m.ShowClientKey(c, true)
		case "apps":
			return m.ShowClientApps(c, true)
		case "guide":
			return m.ShowClientGuide(c, true)
		case "about":
			return m.ShowClientAbout(c, true)
		case "support":
			return m.ShowClientSupport(c, true)
		case "trial_prompt":
			return m.ShowClientTrialPrompt(c, true)
		case "tariffs":
			return m.ShowClientTariffs(c, true)
		}

	case "trial":
		if action == "submit" {
			return m.SubmitTrialRequest(c)
		}

	case "order":
		switch action {
		case "select":
			if len(args) > 0 {
				return m.SelectTariff(c, args[0])
			}
		case "paid":
			if len(args) > 0 {
				return m.PromptReceiptUpload(c, args[0])
			}
		case "cancel":
			if len(args) > 0 {
				return m.CancelOrder(c, args[0])
			}
		}

	case "trial_adm":
		switch action {
		case "approve":
			if len(args) >= 2 {
				days, _ := strconv.Atoi(args[1])
				return m.HandleAdminTrialApprove(c, args[0], days)
			}
		case "custom":
			if len(args) > 0 {
				return m.HandleAdminTrialCustom(c, args[0])
			}
		case "reject":
			if len(args) > 0 {
				return m.HandleAdminTrialReject(c, args[0])
			}
		}

	case "order_adm":
		switch action {
		case "approve":
			if len(args) > 0 {
				return m.HandleAdminOrderApprove(c, args[0])
			}
		case "reject":
			if len(args) > 0 {
				return m.HandleAdminOrderReject(c, args[0])
			}
		}

	default:
		log.Printf("[CALLBACK UNHANDLED] category=%q action=%q args=%v raw=%q unique=%q",
			category, action, args, c.Callback().Data, c.Callback().Unique)
		RespondAlert(c, "Неизвестная команда")
	}

	return nil
}

// handleText обрабатывает ввод текста для FSM-диалогов и Reply-кнопок.
func (m *Manager) handleText(c tele.Context) error {
	text := strings.TrimSpace(c.Text())
	log.Printf("[TEXT] from=%d user=%q text=%q", c.Sender().ID, c.Sender().Username, text)
	if strings.HasPrefix(text, "/") {
		// Команда отменяет текущий FSM state
		m.fsm.Clear(c.Sender().ID)
		return nil
	}

	// Проверка наличия активного FSM-состояния у пользователя (включая загрузку чека)
	state, hasState := m.fsm.Get(c.Sender().ID)
	if hasState && state.Action == "waiting_receipt" {
		return c.Send("Пожалуйста, отправьте чек об оплате в виде фото или скриншота.")
	}

	if !m.bot.IsAdmin(c.Sender().ID) {
		return nil
	}

	// Обработка кнопок нижней клавиатуры (ReplyKeyboard)
	switch text {
	case "👥 Клиенты":
		return m.ShowClientsList(c, 1, "", false)
	case "➕ Новый":
		return m.StartNewClientWizard(c, false)
	case "📊 Статистика":
		return m.ShowStats(c, false)
	case "🔍 Поиск":
		return m.ShowSearchPrompt(c, false)
	}

	// Проверка наличия активного FSM-состояния
	if !hasState {
		// Состояния нет — подскажем админу
		markup := &tele.ReplyMarkup{}
		btnMain := markup.Data("🏠 Главное меню", "menu", "main")
		markup.Inline(markup.Row(btnMain))
		return c.Send("Используйте кнопки меню для управления:", markup)
	}

	switch state.Action {
	case "new_name", "new_days_custom":
		return m.HandleFSMNewText(c, state, text)

	case "clients_filter":
		m.fsm.Clear(c.Sender().ID)
		return m.ShowClientsList(c, 1, text, false)

	case "search_query":
		m.fsm.Clear(c.Sender().ID)
		all := m.bot.ClientsStorage().List()
		client, err, ambiguous := bot.ResolveClient(all, text)
		if err == nil {
			return m.ShowClientCard(c, client, false)
		}
		if len(ambiguous) > 0 {
			return m.ShowClientsList(c, 1, text, false)
		}
		// Ничего не найдено
		textMsg := fmt.Sprintf("По запросу «%s» ничего не найдено.", text)
		markup := &tele.ReplyMarkup{}
		btnSearchAgain := markup.Data("🔍 Искать снова", "menu", "search")
		btnMain := markup.Data("🏠 В меню", "menu", "main")
		markup.Inline(markup.Row(btnSearchAgain, btnMain))
		return c.Send(textMsg, markup, tele.ModeMarkdown)

	case "rename_new":
		return m.HandleFSMRenameText(c, state, text)

	case "extend_days":
		return m.HandleFSMExtendCustomText(c, state, text)

	case "shorten_days":
		return m.HandleFSMShortenCustomText(c, state, text)

	case "trial_custom_days":
		days, err := strconv.Atoi(text)
		if err != nil || days <= 0 {
			return c.Send("Пожалуйста, введите количество дней числом (например, `45`):", tele.ModeMarkdown)
		}
		if days > 3650 {
			days = 3650
		}
		reqID := state.Data["req_id"]
		m.fsm.Clear(c.Sender().ID)
		return m.HandleAdminTrialApprove(c, reqID, days)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Обратная совместимость команд (Command Handlers)
// -----------------------------------------------------------------------------

func (m *Manager) handleStart(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	if m.bot.IsAdmin(c.Sender().ID) {
		return m.ShowMainMenu(c, false)
	}
	return m.ShowClientHome(c, false)
}

func (m *Manager) handleHelp(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	if m.bot.IsAdmin(c.Sender().ID) {
		return m.ShowHelp(c, false)
	}
	return m.ShowClientGuide(c, false)
}

func (m *Manager) handleMenuCommand(c tele.Context) error {
	if !m.bot.IsAdmin(c.Sender().ID) {
		return c.Send("⛔ Эта команда доступна только администратору.")
	}

	args := strings.TrimSpace(c.Data())
	if args == "set persistent" || args == "persistent on" {
		m.persistentReplyKb = true
		return c.Send("Постоянная клавиатура включена.")
	}
	if args == "persistent off" || args == "off" {
		m.persistentReplyKb = false
		rm := &tele.ReplyMarkup{RemoveKeyboard: true}
		return c.Send("Постоянная клавиатура выключена.", rm)
	}

	return m.ShowMainMenu(c, false)
}

func (m *Manager) handleListCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	filter := strings.TrimSpace(c.Data())
	return m.ShowClientsList(c, 1, filter, false)
}

func (m *Manager) handleInfoCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	arg := strings.TrimSpace(c.Data())
	if arg == "" {
		return c.Send("Использование: `/info <имя|uuid>`\nПример: `/info Марина`", tele.ModeMarkdown)
	}
	all := m.bot.ClientsStorage().List()
	client, err, ambiguous := bot.ResolveClient(all, arg)
	if err == nil {
		return m.ShowClientCard(c, client, false)
	}
	if len(ambiguous) > 0 {
		return m.ShowClientsList(c, 1, arg, false)
	}
	return c.Send(fmt.Sprintf("Клиент «%s» не найден.", arg))
}

func (m *Manager) handleNewKeyCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	raw := strings.TrimSpace(c.Data())
	if raw == "" {
		// Без аргументов — запускаем интерактивный мастер создания
		return m.StartNewClientWizard(c, false)
	}

	name, days, _, err := bot.ParseNewKeyArgsDetailed(raw)
	if err != nil {
		return c.Send("Ошибка: " + err.Error() + "\nПример: `/newkey Марина Иванова 365`")
	}

	return m.ExecuteCreateClient(c, name, days)
}

func (m *Manager) handleRenameCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	raw := strings.TrimSpace(c.Data())
	if raw == "" {
		return c.Send("Использование: `/rename <старое> -> <новое>`\nПример: `/rename Марина Иванова -> Марина И.`", tele.ModeMarkdown)
	}

	oldName, newName, err := bot.ParseRenameArgs(raw)
	if err != nil {
		return c.Send("Синтаксис: `/rename <старое> -> <новое>`\nРазделитель — стрелка `->`.")
	}

	all := m.bot.ClientsStorage().List()
	client, err, _ := bot.ResolveClient(all, oldName)
	if err != nil {
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", oldName))
	}

	err = m.bot.ClientsStorage().Rename(client.UUID, newName)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	m.bot.LogAdminAction(c.Sender().ID, "rename", client.UUID, true)
	updated, _ := m.bot.ClientsStorage().Get(client.UUID)
	_ = c.Send(fmt.Sprintf("✅ Клиент переименован: «*%s*» → «*%s*»", client.Name, newName), tele.ModeMarkdown)
	return m.ShowClientCard(c, updated, false)
}

func (m *Manager) handleExtendCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	tokens := strings.Fields(strings.TrimSpace(c.Data()))
	if len(tokens) < 2 {
		return c.Send("Использование: `/extend <имя|uuid> <дни>`\nПример: `/extend Марина 30`", tele.ModeMarkdown)
	}
	days, err := strconv.Atoi(tokens[len(tokens)-1])
	if err != nil || days <= 0 {
		return c.Send("Количество дней должно быть положительным числом.")
	}
	target := strings.Join(tokens[:len(tokens)-1], " ")

	all := m.bot.ClientsStorage().List()
	client, err, _ := bot.ResolveClient(all, target)
	if err != nil {
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", target))
	}

	return m.ExecuteExtend(c, client.UUID, days)
}

func (m *Manager) handleShortenCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	tokens := strings.Fields(strings.TrimSpace(c.Data()))
	if len(tokens) < 2 {
		return c.Send("Использование: `/shorten <имя|uuid> <дни>`\nПример: `/shorten Марина 10`", tele.ModeMarkdown)
	}
	days, err := strconv.Atoi(tokens[len(tokens)-1])
	if err != nil || days <= 0 {
		return c.Send("Количество дней должно быть положительным числом.")
	}
	target := strings.Join(tokens[:len(tokens)-1], " ")

	all := m.bot.ClientsStorage().List()
	client, err, _ := bot.ResolveClient(all, target)
	if err != nil {
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", target))
	}

	return m.ExecuteShorten(c, client.UUID, days, false)
}

func (m *Manager) handleDisableCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	target := strings.TrimSpace(c.Data())
	if target == "" {
		return c.Send("Использование: `/disable <имя|uuid>`", tele.ModeMarkdown)
	}
	all := m.bot.ClientsStorage().List()
	client, err, _ := bot.ResolveClient(all, target)
	if err != nil {
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", target))
	}
	_, err = m.bot.ClientsStorage().SetRevoked(client.UUID, true)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}
	m.bot.LogAdminAction(c.Sender().ID, "disable", client.UUID, true)
	updated, _ := m.bot.ClientsStorage().Get(client.UUID)
	_ = c.Send(fmt.Sprintf("🔴 Клиент *%s* отключён.", updated.Name), tele.ModeMarkdown)
	return m.ShowClientCard(c, updated, false)
}

func (m *Manager) handleEnableCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	target := strings.TrimSpace(c.Data())
	if target == "" {
		return c.Send("Использование: `/enable <имя|uuid>`", tele.ModeMarkdown)
	}
	all := m.bot.ClientsStorage().List()
	client, err, _ := bot.ResolveClient(all, target)
	if err != nil {
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", target))
	}
	_, err = m.bot.ClientsStorage().SetRevoked(client.UUID, false)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}
	m.bot.LogAdminAction(c.Sender().ID, "enable", client.UUID, true)
	updated, _ := m.bot.ClientsStorage().Get(client.UUID)
	_ = c.Send(fmt.Sprintf("🟢 Клиент *%s* включён.", updated.Name), tele.ModeMarkdown)
	return m.ShowClientCard(c, updated, false)
}

func (m *Manager) handleDeleteCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	raw := strings.TrimSpace(c.Data())
	if raw == "" {
		return c.Send("Использование: `/delete <имя|uuid> [--force]`", tele.ModeMarkdown)
	}

	force := false
	if strings.HasSuffix(raw, "--force") {
		force = true
		raw = strings.TrimSpace(strings.TrimSuffix(raw, "--force"))
	}

	all := m.bot.ClientsStorage().List()
	client, err, _ := bot.ResolveClient(all, raw)
	if err != nil {
		return c.Send(fmt.Sprintf("Клиент «%s» не найден.", raw))
	}

	if force {
		_, err := m.bot.ClientsStorage().Delete(client.UUID)
		if err != nil {
			return c.Send("Ошибка при удалении: " + err.Error())
		}
		m.bot.LogAdminAction(c.Sender().ID, "delete_force", client.UUID, true)
		return c.Send(fmt.Sprintf("✅ Клиент *%s* удалён немедленно.", client.Name), tele.ModeMarkdown)
	}

	return m.ShowDeleteConfirmation(c, client.UUID, false)
}

func (m *Manager) handleFindCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	arg := strings.TrimSpace(c.Data())
	if arg == "" {
		return m.ShowSearchPrompt(c, false)
	}
	all := m.bot.ClientsStorage().List()
	client, err, ambiguous := bot.ResolveClient(all, arg)
	if err == nil {
		return m.ShowClientCard(c, client, false)
	}
	if len(ambiguous) > 0 {
		return m.ShowClientsList(c, 1, arg, false)
	}
	return c.Send(fmt.Sprintf("По запросу «%s» ничего не найдено.", arg))
}

func (m *Manager) handleStatusCommand(c tele.Context) error {
	m.fsm.Clear(c.Sender().ID)
	return m.ShowStats(c, false)
}
