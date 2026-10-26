package diagnostics

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sirin/server/internal/bot"
	"sirin/server/internal/relays"
)

// CheckStatus представляет статус проверки.
type CheckStatus string

const (
	StatusOK      CheckStatus = "ok"
	StatusWarn     CheckStatus = "warn"
	StatusFail     CheckStatus = "fail"
	StatusUnknown  CheckStatus = "unknown"
)

// AutoFix содержит действие для автоматического безопасного устранения проблемы.
type AutoFix struct {
	ID          string                       `json:"id"`
	Title       string                       `json:"title"`
	Description string                       `json:"description"`
	Safe        bool                         `json:"safe"`
	Apply       func(ctx context.Context) error `json:"-"`
}

// CheckResult результат отдельной проверки.
type CheckResult struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
	Details string      `json:"details,omitempty"`
	AutoFix *AutoFix    `json:"autofix,omitempty"`
}

// ClientInput данные, собранные от пользователя в диалоге/FSM.
type ClientInput struct {
	Platform   string `json:"platform"`    // "android", "windows", "router", "other"
	IssueType  string `json:"issue_type"`  // "no_connect", "slow", "sites_blocked", "key_issue", "other"
	ClientIP   string `json:"client_ip"`   // внешний IP (из ip.me или api.ipify.org)
	AppVersion string `json:"app_version"` // e.g. "1.3.4", "unknown"
	Transport  string `json:"transport"`   // "udp", "vless", "xhttp", "unknown"
}

// ReleaseProvider предоставляет информацию о последних релизах приложений.
type ReleaseProvider interface {
	GetLatestVersion(platform string) (version string, url string)
}

// TCPProber проверяет TCP-доступность хоста и порта, возвращая задержку (RTT).
type TCPProber func(ctx context.Context, host string, port int, timeout time.Duration) (time.Duration, error)

// Dependencies зависимости диагностического движка.
type Dependencies struct {
	APIURL     string
	HTTPClient *http.Client
	Clients    *bot.ClientsStorage
	Storage    *bot.Storage
	Relays     *relays.Storage
	Releases   ReleaseProvider
	TCPProbe   TCPProber
	IsAdmin    bool
}

// DiagnosticReport итоговый отчёт диагностики.
type DiagnosticReport struct {
	Timestamp  time.Time     `json:"timestamp"`
	ClientUUID string        `json:"client_uuid"` // маскированный UUID
	TelegramID int64         `json:"telegram_id"`
	Input      ClientInput   `json:"input"`
	Checks     []CheckResult `json:"checks"`
	Overall    CheckStatus   `json:"overall"`
	Summary    string        `json:"summary"`
}

// MaskUUID маскирует UUID, оставляя только первые 8 символов.
func MaskUUID(uuid string) string {
	if len(uuid) <= 8 {
		return uuid
	}
	return uuid[:8] + "..."
}

// MaskIP маскирует последний октет IP-адреса.
func MaskIP(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return fmt.Sprintf("%s.%s.%s.xxx", parts[0], parts[1], parts[2])
	}
	return ip
}

// HasAutoFixes проверяет, есть ли применимые авто-фиксы.
func (r *DiagnosticReport) HasAutoFixes() bool {
	for _, c := range r.Checks {
		if c.AutoFix != nil && c.AutoFix.Apply != nil {
			return true
		}
	}
	return false
}

// GetAutoFixes возвращает список всех применимых авто-фиксов.
func (r *DiagnosticReport) GetAutoFixes() []*AutoFix {
	var fixes []*AutoFix
	for _, c := range r.Checks {
		if c.AutoFix != nil && c.AutoFix.Apply != nil {
			fixes = append(fixes, c.AutoFix)
		}
	}
	return fixes
}

// FormatText форматирует отчёт для отправки в Telegram.
func (r *DiagnosticReport) FormatText(isAdmin bool) string {
	var sb strings.Builder

	statusIcon := "🟢"
	statusText := "Все системы в норме"
	if r.Overall == StatusFail {
		statusIcon = "🔴"
		statusText = "Обнаружены проблемы"
	} else if r.Overall == StatusWarn {
		statusIcon = "🟡"
		statusText = "Есть предупреждения"
	}

	sb.WriteString(fmt.Sprintf("%s *Диагностика соединения: %s*\n\n", statusIcon, statusText))
	sb.WriteString(fmt.Sprintf("🔑 Ключ: `%s`\n", MaskUUID(r.ClientUUID)))
	if r.Input.Platform != "" {
		sb.WriteString(fmt.Sprintf("📱 Платформа: %s\n", r.Input.Platform))
	}
	if r.Input.Transport != "" && r.Input.Transport != "unknown" {
		sb.WriteString(fmt.Sprintf("⚡ Транспорт: %s\n", strings.ToUpper(r.Input.Transport)))
	}
	sb.WriteString("\n*Результаты проверок:*\n")

	for _, check := range r.Checks {
		var icon string
		switch check.Status {
		case StatusOK:
			icon = "✅"
		case StatusWarn:
			icon = "⚠️"
		case StatusFail:
			icon = "❌"
		default:
			icon = "ℹ️"
		}
		sb.WriteString(fmt.Sprintf("%s *%s*: %s\n", icon, check.Name, check.Message))
		if check.Details != "" && (isAdmin || !strings.Contains(check.Details, "clients:")) {
			sb.WriteString(fmt.Sprintf("   _%s_\n", check.Details))
		}
	}

	if r.Summary != "" {
		sb.WriteString(fmt.Sprintf("\n💡 *Рекомендация:* %s\n", r.Summary))
	}

	return sb.String()
}
