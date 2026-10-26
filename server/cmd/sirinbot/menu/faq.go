package menu

import (
	"fmt"

	tele "gopkg.in/telebot.v3"
)

// ShowFAQMenu отображает главное меню часто задаваемых вопросов (FAQ).
func (m *Manager) ShowFAQMenu(c tele.Context, edit bool) error {
	text := "📖 *База знаний и частые вопросы (FAQ)*\n\n" +
		"Выберите категорию вопроса с готовыми инструкциями по настройке и решению типичных неполадок:"

	markup := &tele.ReplyMarkup{}
	btnNoConnect := markup.Data("🔌 Не подключается", "faq", "no_connect")
	btnSites := markup.Data("🚫 Сайты не открываются", "faq", "sites")
	btnSpeed := markup.Data("🐢 Медленная скорость", "faq", "speed")
	btnKey := markup.Data("🔑 Проблема с ключом", "faq", "key")
	btnSplit := markup.Data("📱 Split tunneling (обход)", "faq", "split")
	btnRouter := markup.Data("🌐 Роутер (OpenWrt / Keenetic)", "faq", "router")
	btnBack := markup.Data("◀ В меню", "my", "home")

	markup.Inline(
		markup.Row(btnNoConnect, btnSites),
		markup.Row(btnSpeed, btnKey),
		markup.Row(btnSplit),
		markup.Row(btnRouter),
		markup.Row(btnBack),
	)

	if edit {
		return EditOrSend(c, text, markup)
	}
	return c.Send(text, markup, tele.ModeMarkdown)
}

// ShowFAQArticle отображает статью FAQ с кнопками [Помогло] и [Не помогло -> Диагностика].
func (m *Manager) ShowFAQArticle(c tele.Context, topic string) error {
	var title, body, diagIssue string

	switch topic {
	case "no_connect":
		diagIssue = "no_connect"
		title = "🔌 Что делать, если Сирин не подключается?"
		body = "1. **Проверьте базовый интернет:** убедитесь, что без VPN сайты и мессенджеры работают.\n" +
			"2. **Проверьте актуальность приложения:** в меню [📥 Приложения] убедитесь, что у вас установлена последняя версия.\n" +
			"3. **Переустановите ключ:** откройте раздел [🔑 Мой ключ], нажмите кнопку «🕊 Открыть в Сирин» для повторного импорта параметров.\n" +
			"4. **Kill Switch:** если на роутере или телефоне включен режим блокировки без VPN, временно отключите его на 1 минуту и проверьте переподключение.\n" +
			"5. **Смена сети:** попробуйте переключиться с Wi-Fi на мобильный интернет (или наоборот)."

	case "sites":
		diagIssue = "sites_blocked"
		title = "🚫 VPN подключен, но YouTube / сайты не открываются?"
		body = "1. **Проверьте ваш внешний IP:** откройте сайт `api.ipify.org` или `ip.me`. Если там ваш домашний IP провайдера, а не сервер Сирин (Нидерланды) — трафик идёт мимо туннеля.\n" +
			"2. **Смените транспорт в приложении:** в настройках переключите транспорт на **VLESS Reality** (порт 443) или **XHTTP** (порт 8443). На мобильных операторах протокол UDP часто душится ТСПУ.\n" +
			"3. **RuDirect (сайты РФ):** сервисы вроде Госуслуг, Сбера и Кинопоиска могут работать напрямую. Убедитесь, что нужный сайт не попал в список исключений.\n" +
			"4. **Перезапустите приложение:** полностью закройте приложение (смахните из недавних) и включите заново."

	case "speed":
		diagIssue = "slow"
		title = "🐢 Низкая скорость или задержки видео?"
		body = "1. **Замерьте скорость:** сделайте замер на `speedtest.net` с включенным VPN и без него. Задержка до европейского сервера (Нидерланды) в норме составляет 40–70 мс.\n" +
			"2. **Смените протокол соединения:** в приложении переключитесь на протокол **VLESS Reality** — он обеспечивает максимальную пропускную способность потокового 4K видео.\n" +
			"3. **Проверьте нагрузку релея:** в боте нажмите кнопку [🩺 Проверить соединение] — система автоматически оценит задержку (RTT) и загрузку вашего узла.\n" +
			"4. **Роутер:** в панели LuCI убедитесь, что аппаратное ускорение (Hardware Flow Offloading) не сбрасывает пакеты туннеля."

	case "key":
		diagIssue = "key_issue"
		title = "🔑 Вопросы по ключу доступа и продлению"
		body = "1. **Где найти свой ключ:** выполните команду `/mykey` в этом боте или нажмите кнопку [🔑 Мой ключ] в главном меню.\n" +
			"2. **Истек срок подписки:** перейдите в [💳 Продлить доступ] и выберите подходящий тариф. Срок действия ключа продлится автоматически.\n" +
			"3. **Ключ отозван (revoked):** если вы видите статус «Ключ отозван», обратитесь в поддержку через раздел [✉️ Создать тикет].\n" +
			"4. **Несколько устройств:** вы можете добавить отдельный ключ для телефона и роутера по кнопке [➕ Добавить устройство]."

	case "split":
		diagIssue = "other"
		title = "📱 Раздельное туннелирование (Split Tunneling)"
		body = "1. **Android:** перейдите в Настройки приложения → *«Раздельное туннелирование»*. Режим **BYPASS** позволяет пустить выбранные российские приложения (банки, маркетплейсы) напрямую в обход VPN.\n" +
			"2. **Роутер OpenWrt:** перейдите в LuCI → Службы → *Sirin VPN* → раздел *«Маршрутизация»*. Встроенный список RuDirect автоматически направляет трафик автономных систем РФ через прямого провайдера.\n" +
			"3. **Проверка:** при включенном Split Tunneling российские сервисы продолжают работать на полной скорости домашнего интернета."

	case "router":
		diagIssue = "other"
		title = "🌐 Настройка и решение проблем на роутере (OpenWrt / Keenetic)"
		body = "1. **Проверка службы:** в SSH роутера выполните команду `/etc/init.d/sirin status` или `logread -e sirin`.\n" +
			"2. **Проверка фаервола:** убедитесь, что интерфейс `sirin0` включен в зону WAN (`uci show firewall.@zone[1]`).\n" +
			"3. **Kill Switch:** при аварийном обрыве туннеля Kill Switch блокирует незащищенный трафик локальной сети для предотвращения утечек. Это штатная работа защиты.\n" +
			"4. **Переустановка клиента:** обновить службу можно одной командой из инструкции: `wget -O- https://raw.githubusercontent.com/goodbrat-lab/sirin-vpn/master/apps/router/install.sh | sh`."
	}

	markup := &tele.ReplyMarkup{}
	btnHelped := markup.Data("✅ Помогло, спасибо!", "faq", "helped")
	btnNotHelped := markup.Data("❌ Не помогло → Запустить диагностику", "faq", "to_diag", diagIssue)
	btnBack := markup.Data("⬅️ К списку вопросов", "my", "faq")

	markup.Inline(
		markup.Row(btnHelped),
		markup.Row(btnNotHelped),
		markup.Row(btnBack),
	)

	text := fmt.Sprintf("*%s*\n\n%s", title, body)
	return EditOrSend(c, text, markup)
}

// HandleFAQCallback обрабатывает действия внутри раздела FAQ.
func (m *Manager) HandleFAQCallback(c tele.Context, action string, args []string) error {
	RespondSafe(c)

	switch action {
	case "helped":
		markup := &tele.ReplyMarkup{}
		btnMenu := markup.Data("◀ В главное меню", "my", "home")
		markup.Inline(markup.Row(btnMenu))
		return EditOrSend(c, "🎉 *Отлично! Рады, что инструкция помогла.*\n\nПриятного и безопасного пользования Сирин!", markup)

	case "to_diag":
		// Переход в диагностику с предзаполненной проблемой (пропуск Шага 2)
		issue := "other"
		if len(args) > 0 && args[0] != "" {
			issue = args[0]
		}
		return m.startDiagnosticsFromFAQ(c, issue)

	case "no_connect", "sites", "speed", "key", "split", "router":
		return m.ShowFAQArticle(c, action)
	}

	return nil
}

func (m *Manager) startDiagnosticsFromFAQ(c tele.Context, issue string) error {
	userID := c.Sender().ID
	uk, hasKey := m.bot.Storage().Get(userID)
	if !hasKey || uk == nil {
		return m.ShowDiagStep0KeyPrompt(c, true)
	}

	devs := uk.GetDevices()
	if len(devs) == 0 {
		return m.ShowDiagStep0KeyPrompt(c, true)
	}

	// Выбираем первое или единственное устройство
	dev := devs[0]

	m.fsm.Set(userID, "diag_step3_ip", dev.UUID, 0, map[string]string{
		"uuid":         dev.UUID,
		"platform":     dev.Platform,
		"transport":    dev.Transport,
		"version":      dev.AppVersion,
		"device_label": dev.Label,
		"issue":        issue,
	})

	return m.ShowDiagStep3IPCheck(c, true)
}
