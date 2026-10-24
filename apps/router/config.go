package main

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

type SplitTunnelConfig struct {
	Mode        string   `toml:"mode"`          // "bypass" (default) или "only"
	UseRuDirect *bool    `toml:"use_ru_direct"` // по умолчанию true для bypass
	CIDRs       []string `toml:"cidrs"`         // список IP / CIDR
	Domains     []string `toml:"domains"`       // список доменов
}

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
	StaticLadder       []LadderEntry `toml:"ladder"`
	PreferStaticLadder bool          `toml:"prefer_static_ladder,omitempty"` // принудительно использовать [[ladder]] из конфига вместо API

	// Split tunneling (обратная совместимость с v1.1.2)
	RuDirect    *bool              `toml:"ru_direct,omitempty"`
	SplitTunnel *SplitTunnelConfig `toml:"split_tunnel,omitempty"`

	// Kill Switch (блокировка трафика LAN в WAN при обрыве туннеля)
	KillSwitch           *bool    `toml:"kill_switch,omitempty"`
	KillSwitchExceptions []string `toml:"kill_switch_exceptions,omitempty"`
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
	Path      string   `toml:"path,omitempty"`
	Mode      string   `toml:"mode,omitempty"`
	DTLS      bool     `toml:"dtls,omitempty"`
}

// ValidateCIDR проверяет и нормализует строку IPv4 CIDR или одиночный IP.
// Возвращает нормализованную форму (например "1.1.1.1/32" или "10.0.0.0/8").
func ValidateCIDR(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("пустая строка CIDR")
	}
	if !strings.Contains(s, "/") {
		ip := net.ParseIP(s)
		if ip == nil {
			return "", fmt.Errorf("неверный IP-адрес: %q", s)
		}
		if ip.To4() == nil {
			return "", fmt.Errorf("поддерживается только IPv4: %q", s)
		}
		return ip.To4().String() + "/32", nil
	}

	ip, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		return "", fmt.Errorf("неверный формат CIDR %q: %w", s, err)
	}
	if ip.To4() == nil {
		return "", fmt.Errorf("поддерживается только IPv4: %q", s)
	}
	return ipNet.String(), nil
}

// ValidateDomain проверяет корректность доменного имени.
func ValidateDomain(d string) (string, error) {
	d = strings.ToLower(strings.TrimSpace(d))
	if d == "" {
		return "", fmt.Errorf("пустое имя домена")
	}
	if len(d) > 253 {
		return "", fmt.Errorf("доменное имя слишком длинное (> 253 симв.): %q", d)
	}
	if !domainRegex.MatchString(d) {
		return "", fmt.Errorf("недопустимый формат доменного имени: %q", d)
	}
	return d, nil
}

// NormalizeMode проверяет режим split tunneling (bypass или only).
func NormalizeMode(m string) (string, error) {
	m = strings.ToLower(strings.TrimSpace(m))
	if m == "" || m == "bypass" {
		return "bypass", nil
	}
	if m == "only" {
		return "only", nil
	}
	return "", fmt.Errorf("недопустимый режим split_tunnel: %q (допустимы 'bypass' или 'only')", m)
}

// Validate проверяет и нормализует параметры раздельного туннелирования.
func (s *SplitTunnelConfig) Validate() error {
	mode, err := NormalizeMode(s.Mode)
	if err != nil {
		return err
	}
	s.Mode = mode

	var validCIDRs []string
	for _, c := range s.CIDRs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		norm, err := ValidateCIDR(c)
		if err != nil {
			return err
		}
		validCIDRs = append(validCIDRs, norm)
	}
	s.CIDRs = validCIDRs

	var validDomains []string
	for _, d := range s.Domains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		norm, err := ValidateDomain(d)
		if err != nil {
			return err
		}
		validDomains = append(validDomains, norm)
	}
	s.Domains = validDomains

	return nil
}

// IsRuDirectActive возвращает true, если RuDirect активен (только в режиме bypass).
func (c *Config) IsRuDirectActive() bool {
	if c.SplitTunnel == nil {
		return true
	}
	if c.SplitTunnel.Mode != "bypass" {
		return false
	}
	if c.SplitTunnel.UseRuDirect != nil {
		return *c.SplitTunnel.UseRuDirect
	}
	return true
}

// IsKillSwitchEnabled возвращает true, если Kill Switch активен (по умолчанию true).
func (c *Config) IsKillSwitchEnabled() bool {
	if c.KillSwitch == nil {
		return true // default true
	}
	return *c.KillSwitch
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

	// Обратная совместимость для split_tunnel:
	// Если секция [split_tunnel] отсутствует в конфиге, создаём дефолтную секцию.
	if cfg.SplitTunnel == nil {
		defaultUseRu := true
		if cfg.RuDirect != nil {
			defaultUseRu = *cfg.RuDirect
		}
		cfg.SplitTunnel = &SplitTunnelConfig{
			Mode:        "bypass",
			UseRuDirect: &defaultUseRu,
		}
	} else {
		// Если UseRuDirect не указан явно внутри [split_tunnel], смотрим на top-level ru_direct или default=true
		if cfg.SplitTunnel.UseRuDirect == nil {
			defaultUseRu := true
			if cfg.RuDirect != nil {
				defaultUseRu = *cfg.RuDirect
			}
			cfg.SplitTunnel.UseRuDirect = &defaultUseRu
		}
	}

	if err := cfg.SplitTunnel.Validate(); err != nil {
		return nil, fmt.Errorf("ошибка валидации split_tunnel: %w", err)
	}

	return &cfg, nil
}
