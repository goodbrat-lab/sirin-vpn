package tickets

import (
	"time"
)

// TicketStatus статус жизненного цикла тикета.
type TicketStatus string

const (
	StatusOpen       TicketStatus = "open"
	StatusInProgress TicketStatus = "in_progress"
	StatusClosed     TicketStatus = "closed"
)

// MessageSender отправитель сообщения в тикете.
type MessageSender string

const (
	SenderUser  MessageSender = "user"
	SenderAdmin MessageSender = "admin"
)

// TicketMessage сообщение внутри тикета.
type TicketMessage struct {
	From string    `json:"from"` // "user" | "admin"
	Text string    `json:"text"`
	TS   time.Time `json:"ts"`
}

// DiagnosticItem снимок результата диагностической проверки.
type DiagnosticItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warn", "fail", "unknown"
	Message string `json:"message"`
}

// DeviceContext контекст конкретного устройства клиента.
type DeviceContext struct {
	UUID       string `json:"uuid"`
	Label      string `json:"label"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
}

// Ticket модель обращения в техническую поддержку.
type Ticket struct {
	ID               int              `json:"id"`
	TelegramID       int64            `json:"telegram_id"`
	Username         string           `json:"username"`
	DeviceUUID       string           `json:"device_uuid"`
	DeviceLabel      string           `json:"device_label"`
	DevicePlatform   string           `json:"device_platform"`
	DeviceAppVersion string           `json:"device_app_version"`
	Category         string           `json:"category"` // "connection", "key", "speed", "sites", "other"
	Subject          string           `json:"subject,omitempty"`
	Messages         []TicketMessage  `json:"messages"`
	Status           TicketStatus     `json:"status"` // "open", "in_progress", "closed"
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	Diagnostics      []DiagnosticItem `json:"diagnostics,omitempty"`
}

// TicketsFile структура хранения тикетов в JSON.
type TicketsFile struct {
	Tickets []Ticket `json:"tickets"`
}

// MaskUUID возвращает первые 8 символов UUID для безопасного отображения.
func MaskUUID(uuid string) string {
	if len(uuid) <= 8 {
		return uuid
	}
	return uuid[:8] + "..."
}
