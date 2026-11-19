package api

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ListenAddr   string   `toml:"listen_addr"`    // ":8080"
	StoragePath  string   `toml:"storage_path"`   // "/etc/sirin/clients.json"
	AdminToken   string   `toml:"admin_token"`    // Bearer для /v1/admin/*
	ServerHost   string   `toml:"server_host"`    // адрес сервера для /v1/params
	ServerPort   int      `toml:"server_port"`    // 56005
	ServerID     string   `toml:"server_id"`      // "sirin-main"
	Generation   int64    `toml:"generation"`     // 1
	VLESSEnabled bool     `toml:"vless_enabled"`  // включить ли VLESS entry в /v1/params
	VLESSHost    string   `toml:"vless_host"`     // адрес сервера VLESS (e.g. crp.guseynovia.ru)
	VLESSPort    int      `toml:"vless_port"`     // 443
	VLESSPBK     string   `toml:"vless_pbk"`      // Reality Public Key (base64url)
	VLESSSID     string   `toml:"vless_sid"`      // Reality Short ID для VLESS TCP
	VLESSSNIPool []string `toml:"vless_sni_pool"` // пул SNI (первый — основной)
	XHTTPPort    int      `toml:"xhttp_port"`     // 8443
	XHTTPSID     string   `toml:"xhttp_sid"`      // Reality Short ID для XHTTP
	XHTTPPath    string   `toml:"xhttp_path"`     // путь XHTTP (e.g. "/s1r1nxhttp2026k")
	Relay1Host   string   `toml:"relay1_host"`    // адрес Relay 1 (legacy fallback)
	RelaysPath   string   `toml:"relays_path"`    // путь к relays.json
	BotToken     string   `toml:"bot_token"`      // токен Telegram-бота для алертов
	AdminID      int64    `toml:"admin_id"`       // chat_id админа для алертов
}

// DefaultVLESSSNIPool — дефолтный пул SNI для обратной совместимости со старыми sirinapi.toml.
var DefaultVLESSSNIPool = []string{
	"sun8-21.userapi.com",
	"www.kinopoisk.ru",
	"wildberries.ru",
	"www.ikea.com",
}

const (
	defaultVLESSPBK  = "bMDgBlmM7-KUEPbnuqll6ObqzDTVhh_aXeAfsc3N3A4"
	defaultVLESSSID  = "8283f99ec6a0f52e"
	defaultXHTTPSID  = "a4b2c1d3e5f67890"
	defaultXHTTPPath = "/s1r1nxhttp2026k"
)

func (c *Config) effectiveVLESSPBK() string {
	if c.VLESSPBK != "" {
		return c.VLESSPBK
	}
	return defaultVLESSPBK
}

func (c *Config) effectiveVLESSSID() string {
	if c.VLESSSID != "" {
		return c.VLESSSID
	}
	return defaultVLESSSID
}

func (c *Config) effectiveXHTTPSID() string {
	if c.XHTTPSID != "" {
		return c.XHTTPSID
	}
	return defaultXHTTPSID
}

func (c *Config) effectiveXHTTPPath() string {
	if c.XHTTPPath != "" {
		return c.XHTTPPath
	}
	return defaultXHTTPPath
}

func (c *Config) effectiveXHTTPPort() int {
	if c.XHTTPPort > 0 {
		return c.XHTTPPort
	}
	return 8443
}

func (c *Config) effectiveSNIPool() []string {
	if len(c.VLESSSNIPool) > 0 {
		out := make([]string, len(c.VLESSSNIPool))
		copy(out, c.VLESSSNIPool)
		return out
	}
	out := make([]string, len(DefaultVLESSSNIPool))
	copy(out, DefaultVLESSSNIPool)
	return out
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
