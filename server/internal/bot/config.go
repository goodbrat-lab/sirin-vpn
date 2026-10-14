package bot

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	BotToken      string `toml:"bot_token"`
	AdminID       int64  `toml:"admin_id"`
	APIURL        string `toml:"api_url"`         // http://127.0.0.1:8080
	APIAdminToken string `toml:"api_admin_token"` // Bearer для /v1/admin/*
	StoragePath   string `toml:"storage_path"`   // /etc/sirin/bot_users.json
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
	if c.AdminID == 0 {
		return fmt.Errorf("admin_id обязателен")
	}
	if c.APIURL == "" {
		c.APIURL = "http://127.0.0.1:8080"
	}
	if c.StoragePath == "" {
		c.StoragePath = "/etc/sirin/bot_users.json"
	}
	return nil
}
