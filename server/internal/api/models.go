package api

import "time"

// Envelope — общий конверт ответа API.
type Envelope struct {
	Contract string `json:"contract"` // всегда "v1"
	Status   string `json:"status"`   // всегда "answered"
}

// KeyVerdict — ответ /v1/key.
type KeyVerdict struct {
	Contract    string `json:"contract"`
	Status      string `json:"status"`
	Verdict     string `json:"verdict"` // ok|unknown|expired|revoked
	Bound       string `json:"bound"`   // this|other|none
	ExpiresAt   int64  `json:"expires_at"`
	Since       int64  `json:"since"` // unix timestamp создания
	Stale       bool   `json:"stale"`
	StaleAge    int64  `json:"stale_age"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

// ParamsEntry — один узел в /v1/params.
type ParamsEntry struct {
	ID        string   `json:"id"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	DTLS      bool     `json:"dtls"`
	Transport string   `json:"transport"`
	SNI       string   `json:"sni"`
	SNIs      []string `json:"snis,omitempty"`
	PBK       string   `json:"pbk,omitempty"`
	SID       string   `json:"sid,omitempty"`
	Path      string   `json:"path,omitempty"`
	Mode      string   `json:"mode,omitempty"`
}

// ParamsRelay — секция relay в /v1/params.
type ParamsRelay struct {
	Enabled bool   `json:"enabled"` // ОБЯЗАТЕЛЬНО false для нашего сервера
	Target  string `json:"target"`
}

// ParamsLadder — секция ladder. Клиент ожидает: direct_probe_every_attempts, direct_skip_after_fails, second_round_ms.
type ParamsLadder struct {
	DirectProbeEveryAttempts int `json:"direct_probe_every_attempts"`
	DirectSkipAfterFails     int `json:"direct_skip_after_fails"`
	SecondRoundMs            int `json:"second_round_ms"`
	DirectProbeIntervalMs    int `json:"direct_probe_interval_ms,omitempty"`
}

// ParamsResponse — ответ /v1/params.
type ParamsResponse struct {
	Contract   string        `json:"contract"`
	Status     string        `json:"status"`
	Generation int64         `json:"generation"`
	TTL        int           `json:"ttl"`
	Now        int64         `json:"now"`
	Relay      ParamsRelay   `json:"relay"`
	Ladder     ParamsLadder  `json:"ladder"`
	Entries    []ParamsEntry `json:"entries"`
}

// Client — запись клиента в clients.json.
type Client struct {
	UUID      string     `json:"uuid"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // nil = бессрочно
	Revoked   bool       `json:"revoked"`
	DeviceID  string     `json:"device_id,omitempty"`
	LastSeen  *time.Time `json:"last_seen,omitempty"`
	Comment   string     `json:"comment,omitempty"`
	RelayID   int        `json:"relay_id,omitempty"` // 0 = прямой NL, 1 = Relay 1
}

// IsActive возвращает true, если клиент не отозван и срок действия не истёк.
func (c *Client) IsActive() bool {
	if c.Revoked {
		return false
	}
	if c.ExpiresAt != nil && !c.ExpiresAt.IsZero() && c.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

// GetRelayID возвращает relay_id клиента.
func (c *Client) GetRelayID() int {
	return c.RelayID
}

// ClientsFile — корневая структура clients.json.
type ClientsFile struct {
	Version int                `json:"version,omitempty"`
	Clients map[string]*Client `json:"clients"`
}

// AdminCreateRequest — запрос на создание клиента.
type AdminCreateRequest struct {
	Name      string `json:"name"`
	DaysValid int    `json:"days_valid,omitempty"` // 0 = бессрочно
	Comment   string `json:"comment,omitempty"`
}

// AdminCreateResponse — ответ при создании клиента.
type AdminCreateResponse struct {
	Client *Client `json:"client"`
	Key    string  `json:"key"` // тот же UUID для удобства
}

// ErrorResponse — ответ при ошибке.
type ErrorResponse struct {
	Contract string `json:"contract"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
	Message  string `json:"message,omitempty"`
}
