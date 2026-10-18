package main

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	// Подключение
	Gateway  string `toml:"gateway"`   // crp.guseynovia.ru или IP шлюза
	Password string `toml:"password"`  // UUID клиента
	DeviceID string `toml:"device_id"` // router-01

	// TUN
	TunName string `toml:"tun_name"` // sirin0
	TunMTU  int    `toml:"tun_mtu"`  // 1280
	TunIP   string `toml:"tun_ip"`   // 10.77.77.2
	TunMask string `toml:"tun_mask"` // 255.255.255.0

	// Маршрутизация
	LanInterface string `toml:"lan_interface"` // br-lan
	WanInterface string `toml:"wan_interface"` // wan / eth0
	WanGateway   string `toml:"wan_gateway"`   // шлюз WAN провайдера (если не указан, определяется автоматически)

	// API (опционально — если не указан ladder, берём из /v1/params)
	APIURL string `toml:"api_url"` // https://crp.guseynovia.ru

	// Fallback: если API недоступен, использовать статичный ladder
	StaticLadder []LadderEntry `toml:"ladder"`

	// Split tunneling
	RuDirect bool `toml:"ru_direct"` // RU-IP напрямую
}

type LadderEntry struct {
	Name      string   `toml:"name"`
	Transport string   `toml:"transport"` // udp | vless
	Host      string   `toml:"host"`
	Port      int      `toml:"port"`
	SNI       string   `toml:"sni,omitempty"`
	SNIs      []string `toml:"snis,omitempty"`
	PBK       string   `toml:"pbk,omitempty"`
	SID       string   `toml:"sid,omitempty"`
	DTLS      bool     `toml:"dtls,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.TunName == "" {
		cfg.TunName = "sirin0"
	}
	if cfg.TunMTU <= 0 {
		cfg.TunMTU = 1280
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "https://crp.guseynovia.ru"
	}
	if cfg.LanInterface == "" {
		cfg.LanInterface = "br-lan"
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = "router-01"
	}
	return &cfg, nil
}
