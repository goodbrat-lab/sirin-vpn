package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LocalConfig — локальные настройки (сохраняются в %APPDATA%/Sirin/config.json)
type LocalConfig struct {
	Key         string   `json:"key"`          // UUID ключа доступа
	SplitTunnel []string `json:"split_tunnel"` // домены/приложения для split tunneling
	AutoConnect bool     `json:"auto_connect"` // автоподключение при запуске
}

var configPath = func() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.Getenv("USERPROFILE")
	}
	if appData == "" {
		appData = "."
	}
	return filepath.Join(appData, "Sirin", "config.json")
}()

func LoadConfig() (*LocalConfig, error) {
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return &LocalConfig{}, nil // дефолт
	}
	if err != nil {
		return nil, err
	}
	var cfg LocalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(cfg *LocalConfig) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0600)
}
