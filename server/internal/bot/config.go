package bot

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type AdminConfig struct {
	TelegramIDs []int64 `toml:"telegram_ids"`
}

type PaymentConfig struct {
	SBPPhone        string `toml:"sbp_phone"`
	SBPBank         string `toml:"sbp_bank"`
	CardNumber      string `toml:"card_number"`
	RecipientName   string `toml:"recipient_name"`
	SupportUsername string `toml:"support_username"`
}

type Config struct {
	BotToken      string        `toml:"bot_token"`
	AdminID       int64         `toml:"admin_id"`       // legacy single ID
	Admin         AdminConfig   `toml:"admin"`          // [admin] telegram_ids = [...]
	Payment       PaymentConfig `toml:"payment"`        // [payment]
	APIURL        string        `toml:"api_url"`         // http://127.0.0.1:8080
	APIAdminToken string        `toml:"api_admin_token"` // Bearer для /v1/admin/*
	StoragePath   string        `toml:"storage_path"`   // /etc/sirin/bot_users.json
	ClientsPath   string        `toml:"clients_path"`   // /etc/sirin/clients.json
	RelaysPath    string        `toml:"relays_path"`    // /etc/sirin/relays.json
	TicketsPath   string        `toml:"tickets_path"`   // /etc/sirin/tickets.json
}

func LoadConfig(path string) (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) Validate() error {
	if c.BotToken == "" {
		return fmt.Errorf("bot_token обязателен")
	}
	if c.AdminID == 0 && len(c.Admin.TelegramIDs) == 0 {
		return fmt.Errorf("admin_id или admin.telegram_ids обязателен")
	}
	if c.APIURL == "" {
		c.APIURL = "http://127.0.0.1:8080"
	}
	if c.StoragePath == "" {
		c.StoragePath = "/etc/sirin/bot_users.json"
	}
	if c.ClientsPath == "" {
		c.ClientsPath = "/etc/sirin/clients.json"
	}
	if c.RelaysPath == "" {
		c.RelaysPath = "/etc/sirin/relays.json"
	}
	if c.TicketsPath == "" {
		c.TicketsPath = "/etc/sirin/tickets.json"
	}
	if c.Payment.SBPBank == "" {
		c.Payment.SBPBank = "СБП (любой банк)"
	}
	if c.Payment.SupportUsername == "" {
		c.Payment.SupportUsername = "admin"
	}
	return nil
}

func (c *Config) AdminIDs() []int64 {
	seen := make(map[int64]bool)
	var res []int64
	if c.AdminID != 0 {
		seen[c.AdminID] = true
		res = append(res, c.AdminID)
	}
	for _, adminID := range c.Admin.TelegramIDs {
		if !seen[adminID] && adminID != 0 {
			seen[adminID] = true
			res = append(res, adminID)
		}
	}
	return res
}

func (c *Config) IsAdmin(id int64) bool {
	if c.AdminID != 0 && id == c.AdminID {
		return true
	}
	for _, adminID := range c.Admin.TelegramIDs {
		if id == adminID {
			return true
		}
	}
	return false
}
