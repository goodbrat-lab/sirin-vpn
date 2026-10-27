package api

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ListenAddr   string `toml:"listen_addr"`   // ":8080"
	StoragePath  string `toml:"storage_path"`  // "/etc/sirin/clients.json"
	AdminToken   string `toml:"admin_token"`   // Bearer для /v1/admin/*
	ServerHost   string `toml:"server_host"`   // адрес сервера для /v1/params
	ServerPort   int    `toml:"server_port"`   // 56005
	ServerID     string `toml:"server_id"`     // "sirin-main"
	Generation   int64  `toml:"generation"`    // 1
	VLESSEnabled bool   `toml:"vless_enabled"` // включить ли VLESS entry в /v1/params
	VLESSHost    string `toml:"vless_host"`    // адрес сервера VLESS (e.g. crp.guseynovia.ru)
	VLESSPort    int    `toml:"vless_port"`    // 443
	Relay1Host   string `toml:"relay1_host"`   // адрес Relay 1 (legacy fallback)
	RelaysPath   string `toml:"relays_path"`   // путь к relays.json
	BotToken     string `toml:"bot_token"`     // токен Telegram-бота для алертов
	AdminID      int64  `toml:"admin_id"`      // chat_id админа для алертов
}

func LoadConfig(path string) (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if c.ListenAddr == "" {
		c.ListenAddr = ":8080"
	}
	if c.ServerID == "" {
		c.ServerID = "sirin-main"
	}
	if c.Generation == 0 {
		c.Generation = 1
	}
	if c.VLESSPort == 0 {
		c.VLESSPort = 443
	}
	if c.RelaysPath == "" {
		c.RelaysPath = "/etc/sirin/relays.json"
	}
	return &c, nil
}

func (c *Config) Validate() error {
	if c.StoragePath == "" {
		return fmt.Errorf("storage_path обязателен")
	}
	if c.ServerHost == "" {
		return fmt.Errorf("server_host обязателен")
	}
	if c.ServerPort == 0 {
		return fmt.Errorf("server_port обязателен")
	}
	if c.AdminToken == "" {
		return fmt.Errorf("admin_token обязателен")
	}
	return nil
}
