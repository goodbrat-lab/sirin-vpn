package bot

import (
	"fmt"
	"log"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"
)

// ExpiryNotifier отслеживает сроки действия ключей пользователей и отправляет
// напоминания за 3 дня и за 1 день до истечения, а также при наступлении просрочки.
type ExpiryNotifier struct {
	bot      *Bot
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	sendFn   func(to tele.Recipient, what interface{}, opts ...interface{}) (*tele.Message, error)
}

// NewExpiryNotifier создаёт новый экземпляр планировщика напоминаний.
func NewExpiryNotifier(b *Bot, interval time.Duration) *ExpiryNotifier {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	return &ExpiryNotifier{
		bot:      b,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (en *ExpiryNotifier) send(to tele.Recipient, what interface{}, opts ...interface{}) (*tele.Message, error) {
	if en.sendFn != nil {
		return en.sendFn(to, what, opts...)
	}
	if en.bot != nil && en.bot.bot != nil {
		return en.bot.bot.Send(to, what, opts...)
	}
	return nil, fmt.Errorf("bot not initialized")
}

// Start запускает фоновый опрос.
func (en *ExpiryNotifier) Start() {
	en.wg.Add(1)
	go func() {
		defer en.wg.Done()
		ticker := time.NewTicker(en.interval)
		defer ticker.Stop()

		for {
			select {
			case <-en.stopCh:
				return
			case <-ticker.C:
				_, _, _, _ = en.CheckExpirations()
			}
		}
	}()
}

// Stop останавливает фоновый опрос.
func (en *ExpiryNotifier) Stop() {
	close(en.stopCh)
	en.wg.Wait()
}

// CheckExpirations выполняет один цикл проверки подписок и отправки напоминаний.
func (en *ExpiryNotifier) CheckExpirations() (notified3d int, notified1d int, notifiedExp int, err error) {
	if en.bot == nil || en.bot.storage == nil || en.bot.clientsStorage == nil {
		return 0, 0, 0, nil
	}
	if en.sendFn == nil && (en.bot == nil || en.bot.bot == nil) {
		return 0, 0, 0, nil
	}

	users := en.bot.storage.ListUsers()
	now := time.Now().UTC()

	for _, u := range users {
		if u == nil || u.TelegramID == 0 || u.UUID == "" {
			continue
		}

		cl, ok := en.bot.clientsStorage.Get(u.UUID)
		if !ok || cl == nil || cl.Revoked || cl.ExpiresAt == nil || cl.ExpiresAt.IsZero() {
			continue
		}

		diff := cl.ExpiresAt.Sub(now)
		expDateStr := cl.ExpiresAt.Format("02.01.2006")

		// 1. Предупреждение за 3 дня (2 < diff <= 3 дней)
		if diff > 48*time.Hour && diff <= 72*time.Hour {
			// Отправляем, если ещё не отправляли за последние 5 дней
			if u.NotifiedExpiry3dAt == 0 || now.Sub(time.Unix(u.NotifiedExpiry3dAt, 0)) > 5*24*time.Hour {
				markup := &tele.ReplyMarkup{}
				btnStars := markup.Data("⭐ Продлить на 1 мес (75 ⭐)", "stars_buy:1m:"+u.UUID)
				btnTariffs := markup.Data("💳 Все тарифы", "my_stars:"+u.UUID)
				markup.Inline(
					markup.Row(btnStars),
					markup.Row(btnTariffs),
				)

				text := fmt.Sprintf(
					"⏰ *Напоминание о подписке Sirin VPN*\n\n"+
						"Срок действия вашего ключа истекает через *3 дня* (%s).\n\n"+
						"Продлите доступ заранее в 1 клик через Telegram Stars, чтобы защита не прерывалась:",
					expDateStr,
				)

				if _, sendErr := en.send(&tele.User{ID: u.TelegramID}, text, markup, tele.ModeMarkdown); sendErr == nil {
					u.NotifiedExpiry3dAt = now.Unix()
					_ = en.bot.storage.Set(*u)
					notified3d++
					log.Printf("[ExpiryNotifier] 3-day reminder sent to user %d (uuid %s)", u.TelegramID, u.UUID[:8])
				}
			}
			continue
		}

		// 2. Предупреждение за 1 день (0 < diff <= 24 часов)
		if diff > 0 && diff <= 24*time.Hour {
			// Отправляем, если ещё не отправляли за последние 2 дня
			if u.NotifiedExpiry1dAt == 0 || now.Sub(time.Unix(u.NotifiedExpiry1dAt, 0)) > 2*24*time.Hour {
				markup := &tele.ReplyMarkup{}
				btnStars := markup.Data("⭐ Продлить сейчас (75 ⭐)", "stars_buy:1m:"+u.UUID)
				btnTariffs := markup.Data("💳 Все тарифы", "my_stars:"+u.UUID)
				markup.Inline(
					markup.Row(btnStars),
					markup.Row(btnTariffs),
				)

				text := fmt.Sprintf(
					"⚠️ *Внимание: подписка Sirin VPN истекает завтра!*\n\n"+
						"Срок действия вашего ключа заканчивается *%s*.\n\n"+
						"Продлите доступ прямо сейчас, чтобы ваши устройства оставались подключёнными:",
					expDateStr,
				)

				if _, sendErr := en.send(&tele.User{ID: u.TelegramID}, text, markup, tele.ModeMarkdown); sendErr == nil {
					u.NotifiedExpiry1dAt = now.Unix()
					_ = en.bot.storage.Set(*u)
					notified1d++
					log.Printf("[ExpiryNotifier] 1-day reminder sent to user %d (uuid %s)", u.TelegramID, u.UUID[:8])
				}
			}
			continue
		}

		// 3. Уведомление об истечении срока (просрочен менее 24 часов назад)
		if diff <= 0 && diff > -24*time.Hour {
			if u.NotifiedExpiredAt == 0 || now.Sub(time.Unix(u.NotifiedExpiredAt, 0)) > 5*24*time.Hour {
				markup := &tele.ReplyMarkup{}
				btnStars := markup.Data("⭐ Возобновить подписку", "my_stars:"+u.UUID)
				markup.Inline(markup.Row(btnStars))

				text := fmt.Sprintf(
					"⛔ *Срок действия подписки Sirin VPN истёк*\n\n"+
						"Доступ к защищённому туннелю временно приостановлен (%s).\n\n"+
						"Вы можете возобновить доступ в любой момент:",
					expDateStr,
				)

				if _, sendErr := en.send(&tele.User{ID: u.TelegramID}, text, markup, tele.ModeMarkdown); sendErr == nil {
					u.NotifiedExpiredAt = now.Unix()
					_ = en.bot.storage.Set(*u)
					notifiedExp++
					log.Printf("[ExpiryNotifier] expired notification sent to user %d (uuid %s)", u.TelegramID, u.UUID[:8])
				}
			}
		}
	}

	return notified3d, notified1d, notifiedExp, nil
}
