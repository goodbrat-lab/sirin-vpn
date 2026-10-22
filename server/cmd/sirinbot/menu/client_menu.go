package menu

import (
	"fmt"
	"log"
	"strings"
	"time"

	"sirin/server/internal/bot"
	tele "gopkg.in/telebot.v3"
)

// ShowClientHome выводит главный экран личного кабинета клиента.
func (m *Manager) ShowClientHome(c tele.Context, edit bool) error {
	m.fsm.Clear(c.Sender().ID)

	uk, hasKey := m.bot.Storage().Get(c.Sender().ID)
	var client *bot.Client
	if hasKey && uk != nil {
		if cl, ok := m.bot.ClientsStorage().Get(uk.UUID); ok {
			client = cl
		}
	}

	markup := &tele.ReplyMarkup{}

	if client != nil {
		// У клиента есть привязанный ключ
		emoji, statusLabel := ClientStatus(client)
		expStr := "бессрочно"
		daysInfo := ""
		if client.ExpiresAt != nil && !client.ExpiresAt.IsZero() {
			expStr = FormatDateMSK(*client.ExpiresAt)
			if days, ok := client.DaysLeft(); ok {
				if days > 0 {
					daysInfo = fmt.Sprintf(" (осталось %d дн.)", days)
				} else if days == 0 {
					daysInfo = " (истекает сегодня)"
				} else {
					daysInfo = fmt.Sprintf(" (просрочен на %d дн.)", -days)
				}
			}
		}

		text := fmt.Sprintf(
			"🕊 *Личный кабинет Сирин*\n\n"+
				"Статус: *%s %s*\n"+
				"Срок: *до %s*%s\n"+
				"Ключ: `%s`\n\n"+
				"Нажмите *«🕊 Открыть в Сирин»*, чтобы импортировать ключ в приложение в один клик.",
			emoji, statusLabel,
			expStr, daysInfo,
			MaskUUID(client.UUID),
		)

		openURL := fmt.Sprintf("https://crp.guseynovia.ru/key?uuid=%s", client.UUID)
		btnOpen := markup.URL("🕊 Открыть в Сирин", openURL)
		btnKey := markup.Data("🔑 Мой ключ", "my", "key")
		btnTariffs := markup.Data("💳 Продлить доступ", "my", "tariffs")
		btnApps := markup.Data("📥 Приложения", "my", "apps")
		btnGuide := markup.Data("📖 Инструкция", "my", "guide")
		btnSupport := markup.Data("💬 Поддержка", "my", "support")

		markup.Inline(
			markup.Row(btnOpen),
			markup.Row(btnKey, btnTariffs),
			markup.Row(btnApps, btnGuide),
			markup.Row(btnSupport),
		)

		if edit {
			return EditOrSend(c, text, markup)
		}
		return c.Send(text, markup, tele.ModeMarkdown)
	}

	// Ключа нет — проверяем, есть ли активная заявка на тест
	if req, ok := m.bot.Storage().GetPendingTrialRequest(c.Sender().ID); ok {
		text := fmt.Sprintf(
			"🕊 *Добро пожаловать в Сирин!*\n\n"+
				"⏳ *Ваша заявка на бесплатный доступ #%s находится на рассмотрении.*\n\n"+
				"Администратор скоро рассмотрит запрос. Мы пришлём ключ прямо в этот чат сразу после одобрения.",
			req.ID,
		)

		btnTariffs := markup.Data("💳 Тарифы и оплата", "my", "tariffs")
		btnApps := markup.Data("📥 Приложения", "my", "apps")
		btnSupport := markup.Data("💬 Поддержка", "my", "support")

		markup.Inline(
			markup.Row(btnTariffs),
			markup.Row(btnApps, btnSupport),
		)

		if edit {
			return EditOrSend(c, text, markup)
		}
		return c.Send(text, markup, tele.ModeMarkdown)
	}

	// Ключа нет и заявок нет — приветственный экран
	text := "🕊 *Добро пожаловать в Сирин!*\n\n" +
		"Быстрый, приватный и устойчивый к блокировкам VPN-сервис.\n\n" +
		"У вас пока нет активного ключа. Вы можете запросить *бесплатный пробный доступ* или выбрать тариф."

	btnTrial := markup.Data("🎁 Бесплатный тест", "my", "trial_prompt")
	btnTariffs := markup.Data("💳 Тарифы и оплата", "my", "tariffs")
	btnApps := markup.Data("📥 Скачать приложение", "my", "apps")
	btnAbout := markup.Data("📖 О сервисе", "my", "about")
	btnSupport := markup.Data("💬 Поддержка", "my", "support")

	markup.Inline(
		markup.Row(btnTrial, btnTariffs),
		markup.Row(btnApps, btnAbout),
		markup.Row(btnSupport),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowClientKey показывает полный ключ клиента и инструкцию по копированию.
func (m *Manager) ShowClientKey(c tele.Context, edit bool) error {
	uk, hasKey := m.bot.Storage().Get(c.Sender().ID)
	if !hasKey || uk == nil {
		RespondAlert(c, "У вас нет активного ключа")
		return m.ShowClientHome(c, edit)
	}

	client, ok := m.bot.ClientsStorage().Get(uk.UUID)
	if !ok {
		RespondAlert(c, "Ключ не найден в базе")
		return m.ShowClientHome(c, edit)
	}

	openURL := fmt.Sprintf("https://crp.guseynovia.ru/key?uuid=%s", client.UUID)
	deepLink := fmt.Sprintf("sirin://key?uuid=%s", client.UUID)
	text := fmt.Sprintf(
		"🔑 *Ваш персональный ключ доступа:*\n\n"+
			"`%s`\n\n"+
			"_(нажмите на ключ, чтобы скопировать)_\n\n"+
			"• Для автоматической настройки нажмите кнопку *«🕊 Открыть в Сирин»* ниже.\n"+
			"• Deep-link: `%s`\n"+
			"• Либо вставьте скопированный ключ вручную в приложении.",
		client.UUID,
		deepLink,
	)

	markup := &tele.ReplyMarkup{}
	btnOpen := markup.URL("🕊 Открыть в Сирин", openURL)
	btnBack := markup.Data("◀ В меню", "my", "home")

	markup.Inline(
		markup.Row(btnOpen),
		markup.Row(btnBack),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowClientApps выводит ссылки на загрузку клиентских приложений.
func (m *Manager) ShowClientApps(c tele.Context, edit bool) error {
	text := "📥 *Клиентские приложения «Сирин»*\n\n" +
		"Выберите вашу платформу для загрузки и установки:\n\n" +
		"📱 *Android:*\n" +
		"• [Скачать APK (GitHub Releases)](https://github.com/goodbrat-lab/sirin-vpn/releases/latest)\n" +
		"• [Инструкция и проверка](https://goodbrat-lab.github.io/sirin-vpn/ACCEPTANCE_ANDROID.html)\n\n" +
		"💻 *Windows Desktop:*\n" +
		"• [Скачать установщик (.exe)](https://github.com/goodbrat-lab/sirin-vpn/releases)\n\n" +
		"📡 *Wi-Fi Роутер (OpenWrt):*\n" +
		"• [Инструкция по установке (Web)](https://goodbrat-lab.github.io/sirin-vpn/ROUTER.html)\n" +
		"• [Архивы пакетов (GitHub)](https://github.com/goodbrat-lab/sirin-vpn/releases)\n\n" +
		"📚 *База знаний и документация:*\n" +
		"• [https://goodbrat-lab.github.io/sirin-vpn/](https://goodbrat-lab.github.io/sirin-vpn/)\n\n" +
		"После установки приложения нажмите *«🕊 Открыть в Сирин»* в главном меню для импорта ключа."

	markup := &tele.ReplyMarkup{}
	btnAndroid := markup.URL("📱 Android (APK)", "https://github.com/goodbrat-lab/sirin-vpn/releases/latest")
	btnWindows := markup.URL("💻 Windows (.exe)", "https://github.com/goodbrat-lab/sirin-vpn/releases")
	btnRouter := markup.URL("📡 Инструкция Роутер", "https://goodbrat-lab.github.io/sirin-vpn/ROUTER.html")
	btnDocs := markup.URL("📚 Документация", "https://goodbrat-lab.github.io/sirin-vpn/")
	btnBack := markup.Data("◀ В меню", "my", "home")

	markup.Inline(
		markup.Row(btnAndroid, btnWindows),
		markup.Row(btnRouter, btnDocs),
		markup.Row(btnBack),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowClientGuide выводит краткую инструкцию по подключению.
func (m *Manager) ShowClientGuide(c tele.Context, edit bool) error {
	text := "📖 *Как подключиться к Сирин:*\n\n" +
		"1️⃣ *Установите приложение* для вашего устройства (раздел «📥 Приложения»).\n" +
		"2️⃣ *Откройте ключ:* в главном меню бота нажмите кнопку *«🕊 Открыть в Сирин»* — приложение запустится и сохранит ключ автоматически.\n" +
		"3️⃣ *Подключитесь:* нажмите большую круглую кнопку в центре экрана приложения.\n\n" +
		"Если ссылка не открывает приложение, скопируйте ключ из раздела *«🔑 Мой ключ»* и вставьте его в поле ввода приложения вручную."

	markup := &tele.ReplyMarkup{}
	btnBack := markup.Data("◀ В меню", "my", "home")
	markup.Inline(markup.Row(btnBack))

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowClientAbout выводит информацию о сервисе.
func (m *Manager) ShowClientAbout(c tele.Context, edit bool) error {
	text := "🛡 *О сервисе Сирин*\n\n" +
		"«Сирин» спроектирован специально для работы в условиях жёстких интернет-блокировок:\n\n" +
		"• *Маскировка под веб-трафик (VLESS Reality):* провайдер видит только обычные обращения к популярным сайтам.\n" +
		"• *Шифрование Noise NK:* криптографическая стойкость современного уровня.\n" +
		"• *Без логов:* мы не сохраняем историю посещённых вами сайтов и трафик.\n" +
		"• *Высокая скорость:* сервер подключён к скоростному каналу с низким пингом."

	markup := &tele.ReplyMarkup{}
	btnBack := markup.Data("◀ В меню", "my", "home")
	markup.Inline(markup.Row(btnBack))

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowClientSupport выводит контакты поддержки.
func (m *Manager) ShowClientSupport(c tele.Context, edit bool) error {
	supportUser := m.bot.Config().Payment.SupportUsername
	if supportUser == "" {
		supportUser = "admin"
	}
	supportUser = strings.TrimPrefix(supportUser, "@")

	text := fmt.Sprintf(
		"💬 *Служба поддержки*\n\n"+
			"Если у вас возникли вопросы по настройке, оплате или работе VPN:\n\n"+
			"Напишите администратору: @%s\n\n"+
			"Мы всегда рады помочь!",
		supportUser,
	)

	markup := &tele.ReplyMarkup{}
	btnContact := markup.URL("💬 Написать в поддержку", fmt.Sprintf("https://t.me/%s", supportUser))
	btnBack := markup.Data("◀ В меню", "my", "home")

	markup.Inline(
		markup.Row(btnContact),
		markup.Row(btnBack),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// -----------------------------------------------------------------------------
// Бесплатный тестовый доступ (Trial Flow)
// -----------------------------------------------------------------------------

// ShowClientTrialPrompt отображает диалог подтверждения запроса бесплатного доступа.
func (m *Manager) ShowClientTrialPrompt(c tele.Context, edit bool) error {
	uk, hasKey := m.bot.Storage().Get(c.Sender().ID)
	if hasKey && uk != nil {
		if cl, ok := m.bot.ClientsStorage().Get(uk.UUID); ok && cl.IsActive() {
			RespondAlert(c, "У вас уже есть активный ключ доступа!")
			return m.ShowClientHome(c, edit)
		}
	}

	if _, ok := m.bot.Storage().GetPendingTrialRequest(c.Sender().ID); ok {
		RespondAlert(c, "Ваша заявка уже на рассмотрении!")
		return m.ShowClientHome(c, edit)
	}

	text := "🎁 *Бесплатный тестовый доступ*\n\n" +
		"Вы можете запросить бесплатный пробный период для проверки качества и скорости VPN.\n\n" +
		"Администратор рассмотрит заявку и предоставит доступ.\n\n" +
		"Отправить заявку администратору?"

	markup := &tele.ReplyMarkup{}
	btnConfirm := markup.Data("✅ Да, отправить заявку", "trial", "submit")
	btnBack := markup.Data("◀ Назад", "my", "home")

	markup.Inline(
		markup.Row(btnConfirm),
		markup.Row(btnBack),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// SubmitTrialRequest создаёт заявку на бесплатный тест и уведомляет администраторов.
func (m *Manager) SubmitTrialRequest(c tele.Context) error {
	userID := c.Sender().ID
	username := c.Sender().Username
	fullName := strings.TrimSpace(c.Sender().FirstName + " " + c.Sender().LastName)
	if fullName == "" {
		fullName = "Пользователь"
	}

	reqID := fmt.Sprintf("TR-%d", time.Now().Unix()%1000000)
	req := &bot.TrialRequest{
		ID:         reqID,
		TelegramID: userID,
		Username:   username,
		FullName:   fullName,
		CreatedAt:  time.Now(),
		Status:     "pending",
	}

	if err := m.bot.Storage().CreateTrialRequest(req); err != nil {
		RespondAlert(c, "Ошибка сохранения заявки")
		return err
	}

	// Уведомление клиенту
	text := fmt.Sprintf(
		"✅ *Заявка на бесплатный доступ #%s отправлена!*\n\n"+
			"Администратор оповещён и скоро рассмотрит запрос.\n"+
			"Как только доступ будет одобрен, бот пришлёт вам ключ прямо в этот чат.",
		reqID,
	)

	markup := &tele.ReplyMarkup{}
	btnBack := markup.Data("🏠 В личный кабинет", "my", "home")
	markup.Inline(markup.Row(btnBack))

	_ = EditOrSend(c, text, markup)

	// Уведомление всем администраторам
	m.notifyAdminsNewTrial(req)
	return nil
}

// notifyAdminsNewTrial рассылает карточку новой заявки на тест администраторам.
func (m *Manager) notifyAdminsNewTrial(req *bot.TrialRequest) {
	adminIDs := m.bot.Config().AdminIDs()
	if len(adminIDs) == 0 {
		return
	}

	userDisplay := req.FullName
	if req.Username != "" {
		userDisplay += fmt.Sprintf(" (@%s)", req.Username)
	}

	text := fmt.Sprintf(
		"🔔 *Новая заявка на бесплатный доступ!*\n\n"+
			"Заявка: *#%s*\n"+
			"Пользователь: *%s*\n"+
			"Telegram ID: `%d`\n"+
			"Дата: %s MSK\n\n"+
			"Выберите срок бесплатного ключа для выдачи:",
		req.ID,
		userDisplay,
		req.TelegramID,
		FormatDateTimeMSK(req.CreatedAt),
	)

	markup := &tele.ReplyMarkup{}
	btn3 := markup.Data("3 дня", "trial_adm", "approve", req.ID, "3")
	btn7 := markup.Data("7 дней", "trial_adm", "approve", req.ID, "7")
	btn14 := markup.Data("14 дней", "trial_adm", "approve", req.ID, "14")
	btn30 := markup.Data("30 дней", "trial_adm", "approve", req.ID, "30")
	btnCustom := markup.Data("✏️ Свой срок", "trial_adm", "custom", req.ID)
	btnReject := markup.Data("❌ Отклонить", "trial_adm", "reject", req.ID)

	markup.Inline(
		markup.Row(btn3, btn7),
		markup.Row(btn14, btn30),
		markup.Row(btnCustom, btnReject),
	)

	for _, adminID := range adminIDs {
		adminTarget := &tele.User{ID: adminID}
		_, err := m.bot.TeleBot().Send(adminTarget, text, markup, tele.ModeMarkdown)
		if err != nil {
			log.Printf("[SirinBot] Failed to notify admin %d: %v", adminID, err)
		}
	}
}

// HandleAdminTrialApprove одобряет заявку на тест на указанное число дней.
func (m *Manager) HandleAdminTrialApprove(c tele.Context, reqID string, days int) error {
	req, ok := m.bot.Storage().GetTrialRequest(reqID)
	if !ok {
		RespondAlert(c, "Заявка не найдена")
		return nil
	}
	if req.Status != "pending" {
		RespondAlert(c, fmt.Sprintf("Заявка уже обработана (статус: %s)", req.Status))
		return nil
	}

	// Создаём ключ в базе клиентов
	clientName := req.FullName
	if req.Username != "" {
		clientName = req.FullName + " (@" + req.Username + ")"
	}
	// Ограничиваем длину имени
	runes := []rune(clientName)
	if len(runes) > bot.MaxNameLen {
		clientName = string(runes[:bot.MaxNameLen])
	}

	newClient, err := m.bot.ClientsStorage().Add(clientName, days)
	if err != nil {
		RespondAlert(c, "Ошибка создания ключа: "+err.Error())
		return err
	}

	// Привязываем ключ к Telegram ID пользователя
	_ = m.bot.Storage().Set(bot.UserKey{
		TelegramID: req.TelegramID,
		UUID:       newClient.UUID,
		Name:       clientName,
		CreatedAt:  time.Now().Unix(),
	})

	req.Status = "approved"
	req.Days = days
	req.AdminID = c.Sender().ID
	req.UUID = newClient.UUID
	_ = m.bot.Storage().UpdateTrialRequest(req)

	m.bot.LogAdminAction(c.Sender().ID, "trial_approve", newClient.UUID[:8], true)

	// Обновляем сообщение у администратора
	adminText := fmt.Sprintf(
		"✅ *Заявка #%s одобрена!*\n\n"+
			"Пользователь: %s (`%d`)\n"+
			"Выдан ключ на: *%d дней*\n"+
			"UUID: `%s`\n"+
			"Одобрил: @%s",
		req.ID,
		req.FullName,
		req.TelegramID,
		days,
		newClient.UUID,
		c.Sender().Username,
	)
	_ = EditOrSend(c, adminText, &tele.ReplyMarkup{})

	// Уведомляем клиента
	clientTarget := &tele.User{ID: req.TelegramID}
	expStr := "бессрочно"
	if newClient.ExpiresAt != nil {
		expStr = FormatDateMSK(*newClient.ExpiresAt)
	}

	openURL := fmt.Sprintf("https://crp.guseynovia.ru/key?uuid=%s", newClient.UUID)
	clientMarkup := &tele.ReplyMarkup{}
	btnOpen := clientMarkup.URL("🕊 Открыть в Сирин", openURL)
	btnHome := clientMarkup.Data("🏠 В личный кабинет", "my", "home")
	clientMarkup.Inline(
		clientMarkup.Row(btnOpen),
		clientMarkup.Row(btnHome),
	)

	clientMsg := fmt.Sprintf(
		"🎉 *Вам предоставлен бесплатный доступ!*\n\n"+
			"Срок действия: *%d дней* (до %s)\n\n"+
			"Ваш персональный ключ:\n`%s`\n\n"+
			"Нажмите кнопку *«🕊 Открыть в Сирин»*, чтобы сразу начать пользоваться:",
		days,
		expStr,
		newClient.UUID,
	)

	_, _ = m.bot.TeleBot().Send(clientTarget, clientMsg, clientMarkup, tele.ModeMarkdown)
	return nil
}

// HandleAdminTrialReject отклоняет заявку на тест.
func (m *Manager) HandleAdminTrialReject(c tele.Context, reqID string) error {
	req, ok := m.bot.Storage().GetTrialRequest(reqID)
	if !ok {
		RespondAlert(c, "Заявка не найдена")
		return nil
	}
	if req.Status != "pending" {
		RespondAlert(c, fmt.Sprintf("Заявка уже обработана (статус: %s)", req.Status))
		return nil
	}

	req.Status = "rejected"
	req.AdminID = c.Sender().ID
	_ = m.bot.Storage().UpdateTrialRequest(req)

	m.bot.LogAdminAction(c.Sender().ID, "trial_reject", req.ID, true)

	adminText := fmt.Sprintf(
		"❌ *Заявка #%s отклонена.*\nПользователь: %s (`%d`)\nАдминистратор: @%s",
		req.ID,
		req.FullName,
		req.TelegramID,
		c.Sender().Username,
	)
	_ = EditOrSend(c, adminText, &tele.ReplyMarkup{})

	// Уведомляем клиента
	clientTarget := &tele.User{ID: req.TelegramID}
	clientMarkup := &tele.ReplyMarkup{}
	btnTariffs := clientMarkup.Data("💳 Выбрать тариф", "my", "tariffs")
	btnSupport := clientMarkup.Data("💬 Поддержка", "my", "support")
	clientMarkup.Inline(clientMarkup.Row(btnTariffs, btnSupport))

	clientMsg := "К сожалению, администратор отклонил заявку на бесплатный тестовый период.\n\n" +
		"Вы можете выбрать один из доступных тарифов для подключения:"
	_, _ = m.bot.TeleBot().Send(clientTarget, clientMsg, clientMarkup, tele.ModeMarkdown)
	return nil
}

// HandleAdminTrialCustom запрашивает у админа произвольный срок в днях.
func (m *Manager) HandleAdminTrialCustom(c tele.Context, reqID string) error {
	req, ok := m.bot.Storage().GetTrialRequest(reqID)
	if !ok {
		RespondAlert(c, "Заявка не найдена")
		return nil
	}
	if req.Status != "pending" {
		RespondAlert(c, "Заявка уже обработана")
		return nil
	}

	m.fsm.Set(c.Sender().ID, "trial_custom_days", reqID, 0, map[string]string{
		"req_id": reqID,
	})

	text := fmt.Sprintf(
		"✏️ *Укажите срок для заявки #%s* (пользователь: %s)\n\n"+
			"Введите количество дней числом (например, `45` или `90`):",
		req.ID, req.FullName,
	)

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "menu", "main")
	markup.Inline(markup.Row(btnCancel))

	return EditOrSend(c, text, markup)
}

// -----------------------------------------------------------------------------
// Система оплаты (Вариант А: СБП / перевод + подтверждение чека)
// -----------------------------------------------------------------------------

// ShowClientTariffs отображает список доступных тарифов.
func (m *Manager) ShowClientTariffs(c tele.Context, edit bool) error {
	text := "💳 *Тарифы подписки «Сирин»*\n\n" +
		"Выберите подходящий период доступа:\n\n" +
		"• *1 месяц (30 дн.)* — 150 ₽\n" +
		"• *3 месяца (90 дн.)* — 400 ₽ _(скидка 11%)_\n" +
		"• *6 месяцев (180 дн.)* — 750 ₽ _(скидка 17%)_\n" +
		"• *1 год (365 дн.)* — 1 350 ₽ _(скидка 25%)_\n\n" +
		"⚡ _Безлимитный трафик, максимальная скорость, поддержка всех устройств._"

	markup := &tele.ReplyMarkup{}
	btn1m := markup.Data("1 мес — 150 ₽", "order", "select", "1m")
	btn3m := markup.Data("3 мес — 400 ₽", "order", "select", "3m")
	btn6m := markup.Data("6 мес — 750 ₽", "order", "select", "6m")
	btn1y := markup.Data("1 год — 1 350 ₽", "order", "select", "1y")
	btnBack := markup.Data("◀ В меню", "my", "home")

	markup.Inline(
		markup.Row(btn1m, btn3m),
		markup.Row(btn6m, btn1y),
		markup.Row(btnBack),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// SelectTariff создаёт заказ на оплату и выводит платёжные реквизиты.
func (m *Manager) SelectTariff(c tele.Context, tariffID string) error {
	tariff, ok := FindTariff(tariffID)
	if !ok {
		RespondAlert(c, "Неизвестный тариф")
		return m.ShowClientTariffs(c, true)
	}

	userID := c.Sender().ID
	username := c.Sender().Username
	fullName := strings.TrimSpace(c.Sender().FirstName + " " + c.Sender().LastName)
	if fullName == "" {
		fullName = "Пользователь"
	}

	orderID := fmt.Sprintf("ORD-%d", time.Now().Unix()%1000000)
	order := &bot.PaymentOrder{
		ID:         orderID,
		TelegramID: userID,
		Username:   username,
		FullName:   fullName,
		TariffID:   tariff.ID,
		TariffName: tariff.Name,
		Days:       tariff.Days,
		AmountRub:  tariff.PriceRub,
		CreatedAt:  time.Now(),
		Status:     "pending_receipt",
	}

	if err := m.bot.Storage().CreatePaymentOrder(order); err != nil {
		RespondAlert(c, "Ошибка создания заказа")
		return err
	}

	pCfg := m.bot.Config().Payment
	phone := pCfg.SBPPhone
	if phone == "" {
		phone = "Уточняется у администратора"
	}
	bank := pCfg.SBPBank
	if bank == "" {
		bank = "СБП (любой банк)"
	}
	recipient := pCfg.RecipientName
	if recipient == "" {
		recipient = "Получатель платежа"
	}

	text := fmt.Sprintf(
		"🧾 *Оплата заказа #%s*\n\n"+
			"Тариф: *%s* (%d дней)\n"+
			"К оплате: *%d ₽*\n\n"+
			"📲 *Реквизиты для перевода (СБП):*\n"+
			"• Телефон: `%s`\n"+
			"• Банк: *%s*\n"+
			"• Получатель: *%s*\n\n"+
			"⚠️ *В комментарии к переводу укажите:* `%s`\n\n"+
			"После выполнения перевода нажмите кнопку *«🧾 Я оплатил (прикрепить чек)»* и пришлите фото или скриншот квитанции.",
		order.ID,
		order.TariffName,
		order.Days,
		order.AmountRub,
		phone,
		bank,
		recipient,
		order.ID,
	)

	markup := &tele.ReplyMarkup{}
	btnPaid := markup.Data("🧾 Я оплатил (прикрепить чек)", "order", "paid", order.ID)
	btnBack := markup.Data("◀ К тарифам", "my", "tariffs")
	btnCancel := markup.Data("✖️ Отменить заказ", "order", "cancel", order.ID)

	markup.Inline(
		markup.Row(btnPaid),
		markup.Row(btnBack, btnCancel),
	)

	return EditOrSend(c, text, markup)
}

// PromptReceiptUpload переводит пользователя в режим ожидания скриншота чека.
func (m *Manager) PromptReceiptUpload(c tele.Context, orderID string) error {
	order, ok := m.bot.Storage().GetPaymentOrder(orderID)
	if !ok {
		RespondAlert(c, "Заказ не найден")
		return m.ShowClientTariffs(c, true)
	}

	m.fsm.Set(c.Sender().ID, "waiting_receipt", orderID, 0, map[string]string{
		"order_id": order.ID,
	})

	text := fmt.Sprintf(
		"📎 *Прикрепите чек об оплате заказа #%s*\n\n"+
			"Отправьте фото или скриншот квитанции в ответ на это сообщение.\n\n"+
			"Администратор сверит платёж и подписка будет активирована.",
		order.ID,
	)

	markup := &tele.ReplyMarkup{}
	btnCancel := markup.Data("✖️ Отмена", "order", "cancel", order.ID)
	markup.Inline(markup.Row(btnCancel))

	return EditOrSend(c, text, markup)
}

// CancelOrder отменяет заказ.
func (m *Manager) CancelOrder(c tele.Context, orderID string) error {
	m.fsm.Clear(c.Sender().ID)
	order, ok := m.bot.Storage().GetPaymentOrder(orderID)
	if ok && (order.Status == "pending_receipt" || order.Status == "submitted") {
		order.Status = "cancelled"
		_ = m.bot.Storage().UpdatePaymentOrder(order)
	}

	RespondAlert(c, "Заказ отменён")
	return m.ShowClientHome(c, true)
}

// HandleReceiptUploaded обрабатывает полученный скриншот чека от пользователя.
func (m *Manager) HandleReceiptUploaded(c tele.Context) error {
	state, ok := m.fsm.Get(c.Sender().ID)
	if !ok || state.Action != "waiting_receipt" {
		return nil
	}

	orderID := state.Data["order_id"]
	order, ok := m.bot.Storage().GetPaymentOrder(orderID)
	if !ok {
		m.fsm.Clear(c.Sender().ID)
		return c.Send("Заказ не найден. Пожалуйста, оформите заказ заново через /start.")
	}

	var photoFileID string
	if c.Message().Photo != nil {
		photoFileID = c.Message().Photo.FileID
	} else if c.Message().Document != nil {
		photoFileID = c.Message().Document.FileID
	}

	if photoFileID == "" {
		return c.Send("Пожалуйста, отправьте чек в виде изображения (фото или скриншот).")
	}

	order.Status = "submitted"
	order.ReceiptFileID = photoFileID
	_ = m.bot.Storage().UpdatePaymentOrder(order)
	m.fsm.Clear(c.Sender().ID)

	// Ответ клиенту
	text := fmt.Sprintf(
		"✅ *Чек получен!*\n\n"+
			"Заказ: *#%s* (%s, %d ₽)\n\n"+
			"Администратор уже получил квитанцию и проверяет поступление средств.\n"+
			"Обычно проверка занимает несколько минут. Мы сразу пришлём уведомление, когда подписка будет активирована.",
		order.ID, order.TariffName, order.AmountRub,
	)

	markup := &tele.ReplyMarkup{}
	btnHome := markup.Data("🏠 В личный кабинет", "my", "home")
	markup.Inline(markup.Row(btnHome))

	_ = c.Send(text, markup, tele.ModeMarkdown)

	// Уведомление администраторам с фото
	m.notifyAdminsNewOrder(order)
	return nil
}

// notifyAdminsNewOrder отправляет администраторам чек и кнопки подтверждения.
func (m *Manager) notifyAdminsNewOrder(order *bot.PaymentOrder) {
	adminIDs := m.bot.Config().AdminIDs()
	if len(adminIDs) == 0 {
		return
	}

	userDisplay := order.FullName
	if order.Username != "" {
		userDisplay += fmt.Sprintf(" (@%s)", order.Username)
	}

	caption := fmt.Sprintf(
		"💳 *Новая оплата подписки!*\n\n"+
			"Заказ: *#%s*\n"+
			"Пользователь: *%s*\n"+
			"Telegram ID: `%d`\n"+
			"Тариф: *%s* (%d дн.)\n"+
			"Сумма: *%d ₽*\n"+
			"Дата: %s MSK\n\n"+
			"Проверьте поступление перевода по СБП и подтвердите:",
		order.ID,
		userDisplay,
		order.TelegramID,
		order.TariffName,
		order.Days,
		order.AmountRub,
		FormatDateTimeMSK(order.CreatedAt),
	)

	markup := &tele.ReplyMarkup{}
	btnApprove := markup.Data("✅ Подтвердить оплату", "order_adm", "approve", order.ID)
	btnReject := markup.Data("❌ Отклонить", "order_adm", "reject", order.ID)
	markup.Inline(markup.Row(btnApprove, btnReject))

	photo := &tele.Photo{
		File:    tele.File{FileID: order.ReceiptFileID},
		Caption: caption,
	}

	for _, adminID := range adminIDs {
		adminTarget := &tele.User{ID: adminID}
		_, err := m.bot.TeleBot().Send(adminTarget, photo, markup, tele.ModeMarkdown)
		if err != nil {
			log.Printf("[SirinBot] Failed to send order photo to admin %d: %v", adminID, err)
			// fallback текстом, если фото не отправилось
			_, _ = m.bot.TeleBot().Send(adminTarget, caption, markup, tele.ModeMarkdown)
		}
	}
}

// HandleAdminOrderApprove одобряет оплаченный заказ и активирует/продлевает ключ.
func (m *Manager) HandleAdminOrderApprove(c tele.Context, orderID string) error {
	order, ok := m.bot.Storage().GetPaymentOrder(orderID)
	if !ok {
		RespondAlert(c, "Заказ не найден")
		return nil
	}
	if order.Status != "submitted" && order.Status != "pending_receipt" {
		RespondAlert(c, fmt.Sprintf("Заказ уже обработан (статус: %s)", order.Status))
		return nil
	}

	var clientUUID string
	var expDate string

	// Проверяем, есть ли уже ключ у пользователя
	uk, hasKey := m.bot.Storage().Get(order.TelegramID)
	if hasKey && uk != nil {
		if cl, ok := m.bot.ClientsStorage().Get(uk.UUID); ok {
			// Ключ существует — продлеваем срок и включаем при необходимости
			ext, err := m.bot.ClientsStorage().Extend(cl.UUID, order.Days)
			if err != nil {
				RespondAlert(c, "Ошибка продления: "+err.Error())
				return err
			}
			if ext.Revoked {
				_, _ = m.bot.ClientsStorage().SetRevoked(ext.UUID, false)
			}
			clientUUID = ext.UUID
			if ext.ExpiresAt != nil {
				expDate = FormatDateMSK(*ext.ExpiresAt)
			}
		}
	}

	// Если ключа ещё нет — создаём новый
	if clientUUID == "" {
		clientName := order.FullName
		if order.Username != "" {
			clientName = order.FullName + " (@" + order.Username + ")"
		}
		runes := []rune(clientName)
		if len(runes) > bot.MaxNameLen {
			clientName = string(runes[:bot.MaxNameLen])
		}

		newCl, err := m.bot.ClientsStorage().Add(clientName, order.Days)
		if err != nil {
			RespondAlert(c, "Ошибка создания ключа: "+err.Error())
			return err
		}

		_ = m.bot.Storage().Set(bot.UserKey{
			TelegramID: order.TelegramID,
			UUID:       newCl.UUID,
			Name:       clientName,
			CreatedAt:  time.Now().Unix(),
		})

		clientUUID = newCl.UUID
		if newCl.ExpiresAt != nil {
			expDate = FormatDateMSK(*newCl.ExpiresAt)
		}
	}

	order.Status = "approved"
	order.AdminID = c.Sender().ID
	_ = m.bot.Storage().UpdatePaymentOrder(order)

	m.bot.LogAdminAction(c.Sender().ID, "order_approve", clientUUID[:8], true)

	// Обновляем сообщение у администратора
	adminMsg := fmt.Sprintf(
		"✅ *Оплата заказа #%s подтверждена!*\n\n"+
			"Пользователь: %s (`%d`)\n"+
			"Тариф: %s (%d дн.)\n"+
			"Ключ: `%s`\n"+
			"Действует до: *%s*\n"+
			"Администратор: @%s",
		order.ID,
		order.FullName,
		order.TelegramID,
		order.TariffName,
		order.Days,
		clientUUID,
		expDate,
		c.Sender().Username,
	)

	// Если это сообщение с фото, меняем подпись
	_ = EditOrSend(c, adminMsg, &tele.ReplyMarkup{})

	// Уведомляем клиента
	clientTarget := &tele.User{ID: order.TelegramID}
	openURL := fmt.Sprintf("https://crp.guseynovia.ru/key?uuid=%s", clientUUID)
	clientMarkup := &tele.ReplyMarkup{}
	btnOpen := clientMarkup.URL("🕊 Открыть в Сирин", openURL)
	btnHome := clientMarkup.Data("🏠 В личный кабинет", "my", "home")
	clientMarkup.Inline(
		clientMarkup.Row(btnOpen),
		clientMarkup.Row(btnHome),
	)

	clientText := fmt.Sprintf(
		"🎉 *Оплата успешно подтверждена!*\n\n"+
			"Заказ: *#%s*\n"+
			"Тариф: *%s* (+%d дн.)\n"+
			"Подписка активна до: *%s*\n\n"+
			"Ваш персональный ключ:\n`%s`\n\n"+
			"Нажмите кнопку ниже для подключения:",
		order.ID,
		order.TariffName,
		order.Days,
		expDate,
		clientUUID,
	)

	_, _ = m.bot.TeleBot().Send(clientTarget, clientText, clientMarkup, tele.ModeMarkdown)
	return nil
}

// HandleAdminOrderReject отклоняет заказ на оплату.
func (m *Manager) HandleAdminOrderReject(c tele.Context, orderID string) error {
	order, ok := m.bot.Storage().GetPaymentOrder(orderID)
	if !ok {
		RespondAlert(c, "Заказ не найден")
		return nil
	}
	if order.Status != "submitted" && order.Status != "pending_receipt" {
		RespondAlert(c, fmt.Sprintf("Заказ уже обработан (статус: %s)", order.Status))
		return nil
	}

	order.Status = "rejected"
	order.AdminID = c.Sender().ID
	_ = m.bot.Storage().UpdatePaymentOrder(order)

	m.bot.LogAdminAction(c.Sender().ID, "order_reject", order.ID, true)

	adminMsg := fmt.Sprintf(
		"❌ *Оплата заказа #%s отклонена.*\nПользователь: %s (`%d`)\nАдминистратор: @%s",
		order.ID,
		order.FullName,
		order.TelegramID,
		c.Sender().Username,
	)
	_ = EditOrSend(c, adminMsg, &tele.ReplyMarkup{})

	// Уведомляем клиента
	clientTarget := &tele.User{ID: order.TelegramID}
	clientMarkup := &tele.ReplyMarkup{}
	btnSupport := clientMarkup.Data("💬 Поддержка", "my", "support")
	btnTariffs := clientMarkup.Data("💳 Попробовать снова", "my", "tariffs")
	clientMarkup.Inline(clientMarkup.Row(btnSupport, btnTariffs))

	clientText := fmt.Sprintf(
		"❌ *Оплата заказа #%s не была подтверждена.*\n\n"+
			"Если произошла ошибка или платёж уже списался с вашего счёта, пожалуйста, обратитесь в поддержку с квитанцией.",
		order.ID,
	)

	_, _ = m.bot.TeleBot().Send(clientTarget, clientText, clientMarkup, tele.ModeMarkdown)
	return nil
}
