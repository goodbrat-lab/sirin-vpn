package menu

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"sirin/server/internal/relays"
	tele "gopkg.in/telebot.v3"
)

// WarningTracker отслеживает отправку предупреждений, предотвращая спам (не чаще 1 раза в сутки на устройство).
type WarningTracker struct {
	mu          sync.Mutex
	lastWarnMap map[string]time.Time // key = userID:deviceUUID
}

func (w *WarningTracker) ShouldWarn(userID int64, deviceUUID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := fmt.Sprintf("%d:%s", userID, deviceUUID)
	last, ok := w.lastWarnMap[key]
	if !ok || time.Since(last) >= 24*time.Hour {
		w.lastWarnMap[key] = time.Now()
		return true
	}
	return false
}

var defaultWarningTracker = &WarningTracker{lastWarnMap: make(map[string]time.Time)}

// CheckUserProactiveWarnings проверяет состояние устройств пользователя при /start.
// Возвращает текст предупреждений и кнопки для их решения (если есть что показать).
func (m *Manager) CheckUserProactiveWarnings(userID int64) (string, []tele.Row) {
	uk, hasKey := m.bot.Storage().Get(userID)
	if !hasKey || uk == nil {
		return "", nil
	}

	devs := uk.GetDevices()
	if len(devs) == 0 {
		return "", nil
	}

	var warnLines []string
	var actionRows []tele.Row
	markup := &tele.ReplyMarkup{}

	for _, d := range devs {
		cl, ok := m.bot.ClientsStorage().Get(d.UUID)
		if !ok || cl == nil {
			continue
		}

		// 1. Проверка истечения ключа (< 3 дней)
		if cl.ExpiresAt != nil && !cl.ExpiresAt.IsZero() {
			if days, okDays := cl.DaysLeft(); okDays && days <= 3 && cl.IsActive() {
				if defaultWarningTracker.ShouldWarn(userID, d.UUID) {
					warnLines = append(warnLines, fmt.Sprintf("• *%s*: подписка истекает через %d дн.", d.Label, days))
					btnPay := markup.Data(fmt.Sprintf("💳 Продлить «%s»", d.Label), "my", "tariffs")
					actionRows = append(actionRows, markup.Row(btnPay))
				}
			}
		}

		// 2. Проверка релея устройства
		if cl.RelayID > 0 {
			if rs := m.bot.RelaysStorage(); rs != nil {
				r, found := rs.GetByID(cl.RelayID)
				isOverloaded := false
				if found {
					allClients := m.bot.ClientsStorage().List()
					active := relays.CountActiveOnRelay(allClients, r.ID)
					if r.MaxClients > 0 && active >= r.MaxClients {
						isOverloaded = true
					}
				}

				if !found || !r.Enabled || isOverloaded {
					if defaultWarningTracker.ShouldWarn(userID, d.UUID+"_relay") {
						statusReason := "отключен"
						if isOverloaded {
							statusReason = "перегружен"
						}
						warnLines = append(warnLines, fmt.Sprintf("• *%s*: сервер-релей %s", d.Label, statusReason))
						btnReset := markup.Data(fmt.Sprintf("🔄 Переключить «%s» на Direct NL", d.Label), "client", "set_relay", d.UUID, "0")
						actionRows = append(actionRows, markup.Row(btnReset))
					}
				}
			}
		}
	}

	if len(warnLines) == 0 {
		return "", nil
	}

	text := "⚠️ *Обратите внимание:*\n" + strings.Join(warnLines, "\n")
	return text, actionRows
}

// GenerateAdminDigest формирует строку сводки мониторинга для панели управления.
func (m *Manager) GenerateAdminDigest() string {
	now := time.Now()
	allClients := m.bot.ClientsStorage().List()

	expiringKeys := 0
	for _, c := range allClients {
		if !c.Revoked && c.ExpiresAt != nil && !c.ExpiresAt.IsZero() {
			if c.ExpiresAt.After(now) && c.ExpiresAt.Sub(now) < 3*24*time.Hour {
				expiringKeys++
			}
		}
	}

	overloadedRelays := 0
	if rs := m.bot.RelaysStorage(); rs != nil {
		for _, r := range rs.GetEnabled() {
			active := relays.CountActiveOnRelay(allClients, r.ID)
			if r.MaxClients > 0 && active >= r.MaxClients {
				overloadedRelays++
			}
		}
	}

	openTickets := 0
	if ts := m.bot.TicketsStorage(); ts != nil {
		openTickets = len(ts.GetOpen())
	}

	var parts []string
	if expiringKeys > 0 {
		parts = append(parts, fmt.Sprintf("⏳ Истекают: *%d*", expiringKeys))
	}
	if overloadedRelays > 0 {
		parts = append(parts, fmt.Sprintf("⚠️ Релеев перегружено: *%d*", overloadedRelays))
	}
	if openTickets > 0 {
		parts = append(parts, fmt.Sprintf("🆘 Открытых тикетов: *%d*", openTickets))
	}

	if len(parts) == 0 {
		return "🟢 Все системы в норме (нет срочных уведомлений)"
	}
	return strings.Join(parts, " · ")
}
