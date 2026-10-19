package menu

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"sirin/server/internal/bot"
	tele "gopkg.in/telebot.v3"
)

// Московский часовой пояс UTC+3.
var mskLoc = time.FixedZone("MSK", 3*3600)

// FormatDateMSK форматирует дату в часовом поясе Москвы.
func FormatDateMSK(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.In(mskLoc).Format("2006-01-02")
}

// FormatDateTimeMSK форматирует дату и время в часовом поясе Москвы.
func FormatDateTimeMSK(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.In(mskLoc).Format("2006-01-02 15:04")
}

// MaskTelegramID маскирует Telegram ID (например, 1191978271 -> 1191***271).
func MaskTelegramID(id int64) string {
	s := strconv.FormatInt(id, 10)
	if len(s) <= 6 {
		return s
	}
	return s[:4] + "***" + s[len(s)-3:]
}

// ClientStatus возвращает эмодзи статуса и текстовое описание.
func ClientStatus(c *bot.Client) (emoji string, label string) {
	if c.Revoked {
		return "🔴", "отключён"
	}
	now := time.Now()
	if c.ExpiresAt != nil && !c.ExpiresAt.IsZero() {
		if c.ExpiresAt.Before(now) {
			return "⚫", "просрочен"
		}
		if c.ExpiresAt.Sub(now) < 7*24*time.Hour {
			return "⏳", "истекает"
		}
	}
	return "🟢", "активен"
}

// ParseCallback разбирает строку вида "category:action:arg1:arg2" на компоненты.
func ParseCallback(data string) (category string, action string, args []string) {
	parts := strings.Split(data, ":")
	if len(parts) == 0 {
		return "", "", nil
	}
	category = parts[0]
	if len(parts) > 1 {
		action = parts[1]
	}
	if len(parts) > 2 {
		args = parts[2:]
	}
	return category, action, args
}

// CallbackDedup защищает от повторной обработки быстрых двойных кликов.
type CallbackDedup struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func NewCallbackDedup() *CallbackDedup {
	d := &CallbackDedup{
		last: make(map[string]time.Time),
	}
	go d.cleanupLoop()
	return d
}

func (d *CallbackDedup) ShouldProcess(key string, debounce time.Duration) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if prev, exists := d.last[key]; exists {
		if now.Sub(prev) < debounce {
			return false
		}
	}
	d.last[key] = now
	return true
}

func (d *CallbackDedup) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		for k, t := range d.last {
			if now.Sub(t) > 5*time.Minute {
				delete(d.last, k)
			}
		}
		d.mu.Unlock()
	}
}

// BuildDeepLink формирует прямой deep-link и веб-ссылку для ключа.
func BuildDeepLink(host string, uuid string) (deepLink string, webLink string) {
	deepLink = fmt.Sprintf("sirin://key?uuid=%s", uuid)
	if host == "" {
		host = "vpn.example.com"
	}
	webLink = fmt.Sprintf("https://%s/key?uuid=%s", host, uuid)
	return deepLink, webLink
}

// RespondSafe гарантированно закрывает callback spinner в клиенте.
func RespondSafe(c tele.Context, text ...string) {
	if len(text) > 0 && text[0] != "" {
		_ = c.Respond(&tele.CallbackResponse{Text: text[0], ShowAlert: false})
	} else {
		_ = c.Respond()
	}
}

// RespondAlert показывает всплывающее предупреждение пользователю.
func RespondAlert(c tele.Context, text string) {
	_ = c.Respond(&tele.CallbackResponse{Text: text, ShowAlert: true})
}
