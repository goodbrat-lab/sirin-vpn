package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	tele "gopkg.in/telebot.v3"
)

func (b *Bot) handleStart(c tele.Context) error {
	text := "🕊 *Сирин* — Stay unseen\n\n" +
		"Я помогу подключиться к VPN.\n\n" +
		"Команды:\n" +
		"/newkey `<имя>` — создать новый ключ\n" +
		"/mykey — показать мой ключ\n" +
		"/status — статус сервера\n" +
		"/help — справка\n\n" +
		"Для подключения введи свой UUID в приложении «Сирин»."
	return c.Send(text, tele.ModeMarkdown)
}

func (b *Bot) handleHelp(c tele.Context) error {
	text := "*Справка*\n\n" +
		"`/newkey Иван` — создать ключ с именем 'Иван' (30 дней)\n" +
		"`/newkey Марина Иванова 365` — создать на 365 дней\n" +
		"`/mykey` — показать текущий ключ\n" +
		"`/status` — статистика сервера"
	return c.Send(text, tele.ModeMarkdown)
}

func (b *Bot) handleNewKey(c tele.Context) error {
	userID := c.Sender().ID
	// Проверка: только админ (в будущем можно открыть)
	if !b.isAdmin(userID) {
		return c.Send("🔒 Создание ключей доступно только администратору.")
	}

	// Парсинг аргументов: /newkey <имя> [дни]
	rawInput := c.Data()
	if rawInput == "" {
		rawInput = c.Text()
	}

	name, days, clamped, err := parseNewKeyArgsDetailed(rawInput)
	if err != nil {
		return c.Send("Укажи имя: `/newkey <имя> [дни]`\nПример: `/newkey Марина Иванова 365`", tele.ModeMarkdown)
	}

	// Вызов API
	resp, err := b.api.CreateClient(name, days)
	if err != nil {
		return c.Send(fmt.Sprintf("❌ Ошибка API: %v", err))
	}

	// Сохранить в storage
	_ = b.storage.Set(UserKey{
		TelegramID: userID,
		UUID:       resp.Key,
		Name:       name,
		CreatedAt:  time.Now().Unix(),
	})

	// Структурированное логирование (UUID маскирован: первые 8 символов)
	uuidPrefix := resp.Key
	if len(uuidPrefix) > 8 {
		uuidPrefix = uuidPrefix[:8]
	}
	logPayload, _ := json.Marshal(map[string]interface{}{
		"event":       "newkey",
		"name":        name,
		"days":        days,
		"uuid_prefix": uuidPrefix,
		"admin":       fmt.Sprintf("%d", userID),
	})
	log.Println(string(logPayload))

	openURL := fmt.Sprintf("https://crp.guseynovia.ru/key?uuid=%s", resp.Key)
	btn := &tele.ReplyMarkup{}
	btnOpen := btn.URL("🕊 Открыть в Сирин", openURL)
	btn.Inline(btn.Row(btnOpen))

	durationText := fmt.Sprintf("%d дней", days)
	if clamped {
		durationText = fmt.Sprintf("%d дней (срок ограничен 3650)", days)
	}

	text := fmt.Sprintf("✅ *Ключ создан*\n\n"+
		"Имя: `%s`\n"+
		"Срок: %s\n\n"+
		"🔑 Нажми на ключ ниже, чтобы скопировать:\n"+
		"`%s`\n\n"+
		"Или нажми на кнопку ниже, чтобы открыть приложение «Сирин» и применить ключ:\n"+
		"[Открыть в Сирин](%s)", name, durationText, resp.Key, openURL)

	return c.Send(text, btn, tele.ModeMarkdown)
}

func (b *Bot) handleMyKey(c tele.Context) error {
	userID := c.Sender().ID
	uk, ok := b.storage.Get(userID)
	if !ok {
		return c.Send("У тебя пока нет ключа. Используй /newkey Имя")
	}

	openURL := fmt.Sprintf("https://crp.guseynovia.ru/key?uuid=%s", uk.UUID)
	btn := &tele.ReplyMarkup{}
	btnOpen := btn.URL("🕊 Открыть в Сирин", openURL)
	btn.Inline(btn.Row(btnOpen))

	text := fmt.Sprintf("🔑 *Твой ключ*\n\n"+
		"Имя: `%s`\n\n"+
		"Нажми на ключ ниже, чтобы скопировать:\n"+
		"`%s`\n\n"+
		"Или нажми на кнопку ниже, чтобы открыть приложение «Сирин» и применить ключ:\n"+
		"[Открыть в Сирин](%s)", uk.Name, uk.UUID, openURL)

	return c.Send(text, btn, tele.ModeMarkdown)
}

func (b *Bot) handleStatus(c tele.Context) error {
	if !b.isAdmin(c.Sender().ID) {
		return c.Send("🔒 Только для администратора.")
	}

	health, err := b.api.Health()
	if err != nil {
		return c.Send(fmt.Sprintf("❌ API недоступен: %v", err))
	}

	text := fmt.Sprintf("📊 *Статус сервера*\n\n"+
		"API: %v\n"+
		"Клиентов: %v", health["status"], health["clients"])
	return c.Send(text, tele.ModeMarkdown)
}
