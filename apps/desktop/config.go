package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LocalConfig — локальные настройки (сохраняются в %APPDATA%/Sirin/config.json)
type LocalConfig struct {
	Key          string   `json:"key"`                    // UUID ключа доступа
	AutoConnect  bool     `json:"auto_connect"`            // автоподключение при запуске
	RuDirect     bool     `json:"ru_direct"`               // прямой доступ к RU-диапазонам (RuDirect)
	SplitMode    string   `json:"split_mode"`              // "bypass" (дефолт) или "only"
	SplitCIDRs   []string `json:"split_cidrs"`             // кастомные CIDR-диапазоны
	SplitDomains []string `json:"split_domains"`           // кастомные домены для раздельного туннелирования
	DeadDropURLs []string `json:"dead_drop_urls,omitempty"`// резервные источники конфигурации (Яндекс Диск / S3)
	WhitelistFallback *WhitelistFallbackConfig `json:"whitelist_fallback,omitempty"` // аварийный канал через белые списки (WebRTC TURN VK/DION)
	SplitTunnel  []string `json:"split_tunnel,omitempty"`  // обратная совместимость
}

type WhitelistFallbackConfig struct {
	Enabled     bool     `json:"enabled"`
	Mode        string   `json:"mode"`
	GatewayPort int      `json:"gateway_port,omitempty"`
	UseDTLS     bool     `json:"use_dtls,omitempty"`
	TurnBackup  string   `json:"turn_backup,omitempty"`
	TurnRealm   string   `json:"turn_realm,omitempty"`
	RelayTarget string   `json:"relay_target,omitempty"`
	VKHashes    []string `json:"vk_hashes,omitempty"`
	DionSlugs   []string `json:"dion_slugs,omitempty"`
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
		return &LocalConfig{
			RuDirect:  true,
			SplitMode: "bypass",
		}, nil // дефолт
	}
	if err != nil {
		return nil, err
	}
	var cfg LocalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.SplitMode == "" {
		cfg.SplitMode = "bypass"
	}
	// Миграция со старого split_tunnel, если было заполнено
	if len(cfg.SplitTunnel) > 0 && len(cfg.SplitDomains) == 0 {
		cfg.SplitDomains = cfg.SplitTunnel
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
