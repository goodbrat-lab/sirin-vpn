package config

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ListenAddr     string   `toml:"listen_addr"`
	TCPListenAddr  string   `toml:"tcp_listen_addr"` // "127.0.0.1:56006"
	StaticPrivFile string   `toml:"static_priv_file"`
	TunName        string   `toml:"tun_name"`
	TunIP          string   `toml:"tun_ip"`
	TunNetmask     string   `toml:"tun_netmask"`
	ClientSubnet   string   `toml:"client_subnet"`
	AllowedUUIDs   []string `toml:"allowed_uuids"`
	SessionTTL     int      `toml:"session_ttl"` // секунды, по умолчанию 120
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение файла конфигурации: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("разбор TOML: %w", err)
	}

	if cfg.TCPListenAddr == "" {
		cfg.TCPListenAddr = "127.0.0.1:56006"
	}

	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 120
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("ошибка валидации: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.ListenAddr == "" {
		return errors.New("listen_addr не может быть пустым")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return fmt.Errorf("неверный формат listen_addr: %w", err)
	}

	if c.TCPListenAddr != "" {
		if _, _, err := net.SplitHostPort(c.TCPListenAddr); err != nil {
			return fmt.Errorf("неверный формат tcp_listen_addr: %w", err)
		}
	}

	if c.StaticPrivFile == "" {
		return errors.New("static_priv_file не может быть пустым")
	}

	if c.TunIP == "" || net.ParseIP(c.TunIP) == nil {
		return errors.New("tun_ip должен содержать корректный IPv4-адрес")
	}

	if c.TunNetmask == "" || net.ParseIP(c.TunNetmask) == nil {
		return errors.New("tun_netmask должен содержать корректную маску подсети")
	}

	if c.ClientSubnet == "" {
		return errors.New("client_subnet не может быть пустой")
	}
	if _, _, err := net.ParseCIDR(c.ClientSubnet); err != nil {
		return fmt.Errorf("неверный формат client_subnet: %w", err)
	}

	if len(c.AllowedUUIDs) == 0 {
		return errors.New("список allowed_uuids не должен быть пустым")
	}

	return nil
}
