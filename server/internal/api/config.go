package api

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ListenAddr  string `toml:"listen_addr"`  // ":8080"
	StoragePath string `toml:"storage_path"` // "/etc/sirin/clients.json"
	AdminToken  string `toml:"admin_token"`  // Bearer для /v1/admin/*
	ServerHost  string `toml:"server_host"`  // "94.183.159.114" для /v1/params
	ServerPort  int    `toml:"server_port"`  // 56005
	ServerID    string `toml:"server_id"`    // "sirin-main"
	Generation  int64  `toml:"generation"`   // 1
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
